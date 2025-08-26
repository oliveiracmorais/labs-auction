package auction

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/oliveiracmorais/labs-auction/configuration/logger"
	"github.com/oliveiracmorais/labs-auction/internal/entity/auction_entity"
	"github.com/oliveiracmorais/labs-auction/internal/internal_error"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuctionEntityMongo struct {
	Id          string `bson:"_id"`
	ProductName string `bson:"product_name"`
	Category    string `bson:"category"`
	Description string `bson:"description"`
	Condition   int    `bson:"condition"`
	Status      int    `bson:"status"`
	Timestamp   int64  `bson:"timestamp"`
}

type AuctionRepository struct {
	Collection *mongo.Collection
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	return &AuctionRepository{
		Collection: database.Collection("auctions"),
	}
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {

	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   int(auctionEntity.Condition),
		Status:      int(auctionEntity.Status),
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}

	// Timeout proporcional ao contexto de entrada
	insertCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := ar.Collection.InsertOne(insertCtx, auctionEntityMongo)
	if err != nil {
		logger.Error("Falha ao inserir leilão",
			err,
			zap.String("id", auctionEntityMongo.Id))
		return internal_error.NewInternalServerError("Error trying to insert auction: " + err.Error())
	}

	return nil
}

func (ar *AuctionRepository) StartAuctionMonitor(ctx context.Context) {
	logger.Info("Iniciando monitor de leilões")

	go func() {
		// Pequeno delay inicial para dar tempo ao MongoDB
		time.Sleep(100 * time.Millisecond)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				opCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				ar.closeExpiredAuctions(opCtx)
				cancel()
			case <-ctx.Done():
				logger.Info("Auction monitor stopped")
				return
			}
		}
	}()
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

		ar.updateAuctionStatusToCompleted(auction.Id)
	}
}

func (ar *AuctionRepository) updateAuctionStatusToCompleted(auctionId string) {
	filter := bson.M{"_id": auctionId}
	update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ar.Collection.UpdateOne(timeoutCtx, filter, update)
	if err != nil {
		logger.Error("Error closing expired auction", err, zap.String("auction_id", auctionId))
	} else {
		logger.Info("Auction closed by monitor",
			zap.String("auction_id", auctionId),
			zap.Int64("modified_count", result.ModifiedCount))
	}
}

func (ar *AuctionRepository) getAuctionDuration() time.Duration {
	seconds := os.Getenv("AUCTION_CLOSE_SECONDS")
	if seconds == "" {
		seconds = "10"
	}
	val, err := strconv.Atoi(seconds)
	if err != nil {
		return 10 * time.Second
	}
	return time.Duration(val) * time.Second
}
