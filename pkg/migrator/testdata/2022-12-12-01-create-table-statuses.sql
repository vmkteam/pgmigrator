CREATE TABLE "statuses"
(
    "statusId" SERIAL       NOT NULL,
    "title"    varchar(255) NOT NULL,
    "alias"    varchar(64)  NOT NULL,
    CONSTRAINT "statuses_pkey" PRIMARY KEY ("statusId"),
    CONSTRAINT "statuses_alias_key" UNIQUE ("alias")
);
