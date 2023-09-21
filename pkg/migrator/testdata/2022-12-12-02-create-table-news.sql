CREATE TABLE "news"
(
    "newsId"      SERIAL                   NOT NULL,
    "title"       varchar(255)             NOT NULL,
    "preview"     varchar(255),
    "content"     text,
    "categoryId"  int4                     NOT NULL,
    "tagIds"      int4[],
    "createdAt"   timestamp with time zone NOT NULL DEFAULT NOW(),
    "publishedAt" timestamp with time zone,
    "statusId"    int4                     NOT NULL,
    PRIMARY KEY ("newsId")
);

ALTER TABLE "news"
    ADD CONSTRAINT "Ref_news_to_statuses" FOREIGN KEY ("statusId")
        REFERENCES "statuses" ("statusId")
        MATCH SIMPLE
        ON DELETE NO ACTION
        ON UPDATE NO ACTION
        NOT DEFERRABLE;
