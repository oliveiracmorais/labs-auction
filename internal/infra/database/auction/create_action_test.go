package auction

import (
	"context"
	"testing"
	"time"

	"github.com/oliveiracmorais/labs-auction/internal/entity/auction_entity"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateAuction_ClosesAutomatically(t *testing.T) {
	// Setup
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Skip("MongoDB não disponível")
	}
	defer client.Disconnect(ctx)

	db := client.Database("test_auction")
	repo := NewAuctionRepository(db)

	// Cria leilão com duração curta
	auction, _ := auction_entity.CreateAuction("Produto", "Categoria", "Descrição", auction_entity.New)
	auction.Status = auction_entity.Active

	// Define duração curta para teste
	repo.getAuctionDuration()
	defer func() {
		// Restaura (não ideal, mas funciona para teste simples)
	}()

	// Insere leilão
	errCreate := repo.CreateAuction(ctx, auction)
	if errCreate != nil {
		t.Fatalf("Erro ao criar leilão: %v", err)
	}

	// Aguarda o tempo de fechamento + margem
	time.Sleep(15 * time.Second)

	// Verifica se o leilão foi fechado
	var result AuctionEntityMongo
	err = repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&result)
	if err != nil {
		t.Fatalf("Erro ao buscar leilão: %v", err)
	}

	if result.Status != auction_entity.Completed {
		t.Errorf("Esperado status Completed, obtido: %v", result.Status)
	}
}
