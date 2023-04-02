package e2e

import (
	"testing"

	"toxiproxy-poc/internal/e2e/test"
)

func TestPublishTelemetry(t *testing.T) {
	// GIVEN
	stage := test.NewStage(t)

	// WHEN
	stage.SendTelemetryRequest()

	// THEN
	stage.TheResponseIsSuccessful().
		TheNatsMessageWasPublished()
}

func TestServiceResilience(t *testing.T) {
	t.Run("publish NATS message after restoring connection to NATS", func(t *testing.T) {
		stage := test.NewStage(t)

		stage.NatsIsUnavailable().
			SendTelemetryRequest()

		stage.TheResponseIsSuccessful().
			TheNatsMessageWasNotPublished()

		stage.After5Seconds().
			NatsIsAvailable().
			TheNatsMessageWasPublished()
	})

	t.Run("send NATS message after loose NATS connection and then restart service", func(t *testing.T) {
		stage := test.NewStageWith3NatsReconnect(t)

		stage.NatsIsUnavailable().
			SendTelemetryRequest()

		stage.TheResponseIsSuccessful().
			TheNatsMessageWasNotPublished()

		stage.After5Seconds().
			TheNatsMessageWasNotPublished().
			NatsIsAvailable().
			TheNatsMessageWasNotPublished()

		stage.SendTelemetryRequest().
			TheResponseIsInternalServerError().
			TheNatsMessageWasNotPublished()

		stage.RestartService().
			SendTelemetryRequest().
			TheResponseIsSuccessful().
			TheNatsMessageWasPublished()
	})
}
