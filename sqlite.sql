CREATE TABLE IF NOT EXISTS `users` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `name` TEXT NOT NULL,
  `sid` TEXT NOT NULL
);

INSERT INTO `users` (`name`, `sid`) VALUES ('ed', 'a123456789');