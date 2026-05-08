package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(
	ctx context.Context,
	logger runtime.Logger,
	db *sql.DB,
	nk runtime.NakamaModule,
	initializer runtime.Initializer,
) error {

	logger.Info("Module loaded!")

	err := initializer.RegisterRpc("ping", PingRPC)
	if err != nil {
		return err
	}

	return nil
}

func PingRPC(
	ctx context.Context,
	logger runtime.Logger,
	db *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {

	logger.Info("Ping RPC called")

	return `{"message":"pong"}`, nil
}