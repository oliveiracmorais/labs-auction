package auction

import (
	"context"
	"testing"
	"time"

	"github.com/oliveiracmorais/labs-auction/internal/entity/auction_entity"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateAuction_ClosesAutomatically(t *testing.T) {
	t.Log("🟢 Início do teste")

	// Configura variáveis de ambiente
	t.Setenv("AUCTION_CLOSE_SECONDS", "2")
	t.Setenv("MONGO_INITDB_ROOT_USERNAME", "admin")
	t.Setenv("MONGO_INITDB_ROOT_PASSWORD", "secret")

	// Contextos com timeouts realistas
	mainCtx, cancelMain := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelMain()

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	defer cancelMonitor()

	// URI de conexão
	uri := "mongodb://admin:secret@127.0.0.1:27017/test_auction?authSource=admin&authMechanism=SCRAM-SHA-1"

	client, err := mongo.Connect(mainCtx, options.Client().ApplyURI(uri))
	require.NoError(t, err, "Falha ao conectar ao MongoDB")
	defer func() { _ = client.Disconnect(context.Background()) }()

	// Ping com timeout curto
	require.Eventually(t, func() bool {
		return client.Ping(mainCtx, nil) == nil
	}, 5*time.Second, 500*time.Millisecond, "Falha ao pingar MongoDB")

	db := client.Database("test_auction")
	repo := NewAuctionRepository(db)
	repo.StartAuctionMonitor(monitorCtx)

	collection := repo.Collection
	_, _ = collection.DeleteMany(mainCtx, bson.M{})

	// Cria leilão
	auction, err := auction_entity.CreateAuction("Produto", "Categoria", "Descrição", auction_entity.New)
	require.Nil(t, err)
	auction.Status = auction_entity.Active

	// Insere
	internalErr := repo.CreateAuction(mainCtx, auction)
	require.Nil(t, internalErr, "CreateAuction should not return error")

	// Valida fechamento automático
	require.Eventually(t, func() bool {
		var result AuctionEntityMongo
		err := collection.FindOne(mainCtx, bson.M{"_id": auction.Id}).Decode(&result)
		return err == nil && result.Status == int(auction_entity.Completed)
	}, 5*time.Second, 200*time.Millisecond, "O leilão deveria ter sido fechado automaticamente")
}
