package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func RegisterAuthHooks(initializer runtime.Initializer) error {
	if err := initializer.RegisterAfterAuthenticateApple(afterAuthenticateApple); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateCustom(afterAuthenticateCustom); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateDevice(afterAuthenticateDevice); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateEmail(afterAuthenticateEmail); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateFacebook(afterAuthenticateFacebook); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateFacebookInstantGame(afterAuthenticateFacebookInstantGame); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateGameCenter(afterAuthenticateGameCenter); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateGoogle(afterAuthenticateGoogle); err != nil {
		return err
	}
	if err := initializer.RegisterAfterAuthenticateSteam(afterAuthenticateSteam); err != nil {
		return err
	}

	return nil
}

func afterAuthenticateApple(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateAppleRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateCustom(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateCustomRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateDevice(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateDeviceRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateEmail(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateEmailRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateFacebook(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateFacebookRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateFacebookInstantGame(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateFacebookInstantGameRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateGameCenter(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateGameCenterRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateGoogle(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateGoogleRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func afterAuthenticateSteam(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, out *api.Session, _ *api.AuthenticateSteamRequest) error {
	return handleAfterAuthenticate(ctx, logger, nk, out)
}

func handleAfterAuthenticate(
	ctx context.Context,
	logger runtime.Logger,
	nk runtime.NakamaModule,
	out *api.Session,
) error {
	if out == nil || !out.GetCreated() {
		return nil
	}

	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		logger.Error("Cannot init player resources: missing user_id in runtime context")
		return nil
	}

	if err := initPlayerResources(ctx, nk, userID); err != nil {
		logger.Error("Init player resources failed for user %s: %v", userID, err)
		return err
	}

	logger.Info("Initialized default resources for user %s", userID)
	return nil
}
