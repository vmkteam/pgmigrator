CREATE TABLE "tags"
(
    "tagId"    SERIAL       NOT NULL,
    "title"    varchar(255) NOT NULL,
    "statusId" int4         NOT NULL,
    PRIMARY KEY ("tagId")
);

ALTER TABLE "tags"
    ADD CONSTRAINT "Ref_tags_to_statuses" FOREIGN KEY ("statusId")
        REFERENCES "statuses" ("statusId")
        MATCH SIMPLE
        ON DELETE NO ACTION
        ON UPDATE NO ACTION
        NOT DEFERRABLE;
