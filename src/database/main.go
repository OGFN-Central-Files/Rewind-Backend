package database

import (
	"context"
	"go-backend/src/services"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func StartDatabase() {
	funcs := &services.Functions{}
	connectionURI := "mongodb+srv://kenjikeuhh:BWiukPFSAqr6byTK@cluster0.ic8wv.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0"
	clientOptions := options.Client().ApplyURI(connectionURI)

	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		funcs.Log(services.ERROR, "Database", "Database connection failed")
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			funcs.Log(services.WARNING, "Database", "Database disconnected!")
		}
	}()

	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("MongoDB ping failed: ", err)
	}

	funcs.Log(services.INFO, "Database", "Connected to Database!")
}
