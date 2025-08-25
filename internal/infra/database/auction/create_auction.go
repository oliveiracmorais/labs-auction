package auction

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/oliveiracmorais/labs-auction/configuration/logger"
	"github.com/oliveiracmorais/labs-auction/internal/entity/auction_entity"
	"github.com/oliveiracmorais/labs-auction/internal/internal_error"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection *mongo.Collection
	mu         sync.Mutex
	timers     map[string]*time.Timer
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	repo := &AuctionRepository{
		Collection: database.Collection("auctions"),
		timers:     make(map[string]*time.Timer),
	}
	go repo.cleanupTimers()
	return repo
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	ar.scheduleAuctionClose(auctionEntity.Id)

	return nil
}

func (ar *AuctionRepository) StartAuctionMonitor(ctx context.Context) {
	interval := 30 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ar.closeExpiredAuctions(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (ar *AuctionRepository) closeExpiredAuctions(ctx context.Context) {
	now := time.Now().Unix()
	duration := ar.getAuctionDuration().Seconds()
	expiredTimestamp := now - int64(duration)

	filter := bson.M{
		"status":    auction_entity.Active,
		"timestamp": bson.M{"$lt": expiredTimestamp},
	}

	cursor, err := ar.Collection.Find(ctx, filter)
	if err != nil {
		logger.Error("Erro ao buscar leilões vencidos", err)
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var auction AuctionEntityMongo
		if err := cursor.Decode(&auction); err != nil {
			continue
		}

		ar.scheduleAuctionClose(auction.Id) // ou atualiza diretamente
	}
}

func (ar *AuctionRepository) scheduleAuctionClose(auctionId string) {
	duration := ar.getAuctionDuration()

	timer := time.AfterFunc(duration, func() {
		ar.mu.Lock()
		defer ar.mu.Unlock()

		delete(ar.timers, auctionId)

		filter := bson.M{"_id": auctionId}
		update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := ar.Collection.UpdateOne(timeoutCtx, filter, update)
		if err != nil {
			logger.Error("Error closing auction automatically",
				err,
				zap.String("auction_id", auctionId))
		} else {
			logger.Info("Auction closed automatically",
				zap.String("auction_id", auctionId))
		}
	})

	ar.mu.Lock()
	ar.timers[auctionId] = timer
	ar.mu.Unlock()
}

func (ar *AuctionRepository) getAuctionDuration() time.Duration {
	seconds := os.Getenv("AUCTION_CLOSE_SECONDS")
	if seconds == "" {
		seconds = "10" // valor padrão
	}
	val, err := strconv.Atoi(seconds)
	if err != nil {
		return 10 * time.Second
	}
	return time.Duration(val) * time.Second
}

func (ar *AuctionRepository) cleanupTimers() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ar.mu.Lock()
		for id, timer := range ar.timers {
			if !timer.Stop() {
				// Timer já disparou, remove do mapa
				delete(ar.timers, id)
			}
		}
		ar.mu.Unlock()
	}
}
