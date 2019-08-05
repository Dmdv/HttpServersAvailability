# HttpServersAvailability
Checks servers availability

![](about.jpg)

1. Create database in PostgreSQL
2. Run servers.sql against the created database
3. Update settings.yaml with connection string in folders ServerStat and StatBrowser
4. Build and run main.go in folders ServerStat and StatBrowser

P.S. By default the scheduler checks servers each minute. Browser will check servers status each 2 secs<br>
P.S.S. You don't need to refresh the browser. It will update itself
