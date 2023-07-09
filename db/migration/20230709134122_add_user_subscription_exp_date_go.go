package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upAddUserSubscriptionExpDateGo, downAddUserSubscriptionExpDateGo)
}
func upAddUserSubscriptionExpDateGo(tx *sql.Tx) error {
	// This code is executed when the migration is applied.
	_, err := tx.Exec(`
	ALTER TABLE "user" ADD COLUMN "subscription_exp_date" datetime DEFAULT NULL;
`)
	return err

}

func downAddUserSubscriptionExpDateGo(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	_, err := tx.Exec(`
	ALTER TABLE "user" DROP COLUMN "subscription_exp_date";
`)
	return err
}
