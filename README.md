# briefpg
Temporary PostgreSQL Instances for Unit Tests

briefpg makes it easy to create Go tests backed by a live, temporary Postgres database.  While mocking a database is helpful in some cases, it's just as often helpful to have a real live database to work against.

This project is based on the concepts from the very nice package [ephemeralpg](https://github.com/eradman/ephemeralpg/) by Eric Radman.  Perhaps we should have called it `gophemeralpg`? 

The author also wishes to express gratitude to my employer Brightgate, which allowed its release to Open Source.  And to [Danek Duvall](https://github.com/dhduvall) who helped to review, refine, and fix bugs (unfortunately some of the commit history is lost in the transition to Open Source).
