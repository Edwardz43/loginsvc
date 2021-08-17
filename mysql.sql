CREATE TABLE users (
    `id`   int auto_increment PRIMARY KEY,
    `name` VARCHAR(50) NOT NULL,
    `sid`  VARCHAR(50) NOT NULL,
    CONSTRAINT users_name_uindex UNIQUE (name),
    CONSTRAINT Users_sid_uindex UNIQUE (sid)
);

INSERT INTO `users` (`name`, `sid`) VALUES ('ed', 'a123456789');