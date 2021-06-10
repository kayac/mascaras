BEGIN;

update users
set name = md5(name);

COMMIT;
