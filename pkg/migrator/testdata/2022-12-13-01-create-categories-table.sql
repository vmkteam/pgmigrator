CREATE TABLE "categories"
(
    "categoryId"  SERIAL       NOT NULL,
    "title"       varchar(255) NOT NULL,
    "orderNumber" int4         NOT NULL,
    "statusId"    int4         NOT NULL,
    PRIMARY KEY ("categoryId")
);

ALTER TABLE "news"
    ADD CONSTRAINT "Ref_news_to_categories" FOREIGN KEY ("categoryId")
        REFERENCES "categories" ("categoryId")
        MATCH SIMPLE
        ON DELETE NO ACTION
        ON UPDATE NO ACTION
        NOT DEFERRABLE;

ALTER TABLE "categories"
    ADD CONSTRAINT "Ref_categories_to_statuses" FOREIGN KEY ("statusId")
        REFERENCES "statuses" ("statusId")
        MATCH SIMPLE
        ON DELETE NO ACTION
        ON UPDATE NO ACTION
        NOT DEFERRABLE;
