-- Table Definition
CREATE TABLE "public"."short_urls" (
    "id" int8 NOT NULL,
    "original_url" text NOT NULL,
    "short_url" text NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "hits" int8 NOT NULL DEFAULT '0'::bigint,
    "creator_ip" text NOT NULL DEFAULT ''::text,
    "creator_user_agent" text NOT NULL DEFAULT ''::text,
    PRIMARY KEY ("id")
);
