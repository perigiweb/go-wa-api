package store

import (
	"database/sql"
	"log"
)

type migrateFunc func(*sql.Tx) error

var migrates = [...]migrateFunc{migrateV1}

func (r *Repo) getMigrateVersion() (int, error) {
	_, err := r.db.Exec("CREATE TABLE IF NOT EXISTS migrations (version INTEGER)")
	if err != nil {
		return -1, err
	}

	version := 0
	row := r.db.QueryRow("SELECT version FROM migrations LIMIT 1")
	if row != nil {
		_ = row.Scan(&version)
	}

	return version, nil
}

func (r *Repo) setMigrateVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec("DELETE FROM migrations")
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO migrations (version) VALUES ($1)", version)
	return err
}

func (r *Repo) Migrate() error {
	version, err := r.getMigrateVersion()
	if err != nil {
		return err
	}

	for ; version < len(migrates); version++ {
		var tx *sql.Tx

		tx, err = r.db.Begin()
		if err != nil {
			return err
		}

		log.Printf("Migrate DB to version v%d", version+1)

		mf := migrates[version]
		err = mf(tx)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if err = r.setMigrateVersion(tx, version+1); err != nil {
			return err
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func migrateV1(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE SEQUENCE users_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "users" (
		"id" integer DEFAULT nextval('users_id_seq') NOT NULL,
		"email" character varying(255) NOT NULL,
		"password" character varying(255) NOT NULL,
		"name" character varying(255) NOT NULL,
		"status" smallint NOT NULL,
		"registered_at" timestamptz NOT NULL,
		"updated_at" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT "users_email" UNIQUE ("email"),
		CONSTRAINT "users_pkey" PRIMARY KEY ("id")
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_sessions" (
		"id" uuid DEFAULT gen_random_uuid() NOT NULL,
		"user_id" integer NOT NULL,
		"created_at" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
		"status" boolean DEFAULT true NOT NULL,
		"user_agent" character varying(255) NOT NULL,
		"ip" character varying(128) NOT NULL,
		"expired_at" integer NOT NULL,
		CONSTRAINT "user_sessions_id" PRIMARY KEY ("id"),
		CONSTRAINT "user_sessions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_devices" (
		"id" uuid DEFAULT gen_random_uuid() NOT NULL,
		"user_id" integer NOT NULL,
		"name" character varying(64) NOT NULL,
		"created_at" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
		"jid" text,
		"connected" boolean DEFAULT false NOT NULL,
		CONSTRAINT "user_devices_id" PRIMARY KEY ("id"),
		CONSTRAINT "user_devices_jid" UNIQUE ("jid"),
		CONSTRAINT "user_devices_userId_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE SEQUENCE user_contacts_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_contacts" (
		"id" integer DEFAULT nextval('user_contacts_id_seq') NOT NULL,
		"name" character varying(64),
		"phone" character varying(16),
		"user_id" integer,
		"in_wa" smallint DEFAULT '0' NOT NULL,
		"verified_name" character varying(255),
		CONSTRAINT "user_contacts_phone_user" UNIQUE ("phone", "user_id"),
		CONSTRAINT "user_contacts_pkey" PRIMARY KEY ("id"),
		CONSTRAINT "user_contact_groups_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE RESTRICT ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE SEQUENCE user_contact_groups_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_contact_groups" (
    "id" integer DEFAULT nextval('user_contact_groups_id_seq') NOT NULL,
    "name" character varying(32) NOT NULL,
    "user_id" integer,
    CONSTRAINT "user_contact_groups_name" UNIQUE ("name"),
    CONSTRAINT "user_contact_groups_pkey" PRIMARY KEY ("id"),
		CONSTRAINT "user_contact_groups_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE RESTRICT ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_contact_groups_contacts" (
		"group_id" integer NOT NULL,
		"contact_id" integer NOT NULL,
		CONSTRAINT "user_contact_group_id_contact_id" PRIMARY KEY ("group_id", "contact_id"),
		CONSTRAINT "user_contact_groups_contacts_contact_id_fkey" FOREIGN KEY (contact_id) REFERENCES user_contacts(id) ON UPDATE RESTRICT ON DELETE CASCADE NOT DEFERRABLE,
		CONSTRAINT "user_contact_groups_contacts_group_id_fkey" FOREIGN KEY (group_id) REFERENCES user_contact_groups(id) ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE SEQUENCE user_broadcasts_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_broadcasts" (
    "id" bigint DEFAULT nextval('user_broadcasts_id_seq') NOT NULL,
    "user_id" integer NOT NULL,
    "message" text NOT NULL,
    "media" jsonb,
    "contact_type" character(1) NOT NULL,
    "contact_filter" character(1) NOT NULL,
    "filter_value" character varying(128) NOT NULL,
    "phones" jsonb,
    "jid" TEXT,
    "completed" boolean DEFAULT false NOT NULL,
    "created_at" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    "completed_at" timestamptz,
    "updated_at" timestamptz,
    "campaign_name" character varying(255) NOT NULL,
    "sent_started_at" timestamptz,
    CONSTRAINT "user_broadcasts_pkey" PRIMARY KEY ("id"),
		CONSTRAINT "user_broadcasts_jid_fkey" FOREIGN KEY (jid) REFERENCES user_devices(jid) ON UPDATE CASCADE NOT DEFERRABLE,
		CONSTRAINT "user_broadcasts_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE SEQUENCE user_broadcast_recipients_id_seq INCREMENT 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE "user_broadcast_recipients" (
    "id" bigint DEFAULT nextval('user_broadcast_recipients_id_seq') NOT NULL,
    "broadcast_id" bigint NOT NULL,
    "phone" text NOT NULL,
    "name" character varying(64) NOT NULL,
    "sent_status" character varying(32),
    "sent_at" timestamptz,
    "message_id" text,
    CONSTRAINT "user_broadcast_recipients_pkey" PRIMARY KEY ("id"),
		CONSTRAINT "user_broadcast_recipients_broadcast_id_fkey" FOREIGN KEY (broadcast_id) REFERENCES user_broadcasts(id) ON DELETE CASCADE NOT DEFERRABLE
	)`)
	if err != nil {
		return err
	}

	return nil
}
