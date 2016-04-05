# extrahop-backup

Stores ExtraHop configuration backups in a Git repository.

It currently grabs the ExtraHop configuration through the ExtraHop API, and it grabs the following endpoints:

- runningconfig
- triggers
- auditlog

It takes these endpoints, dumps the outputted data into a git repository, and commits the files.

It runs on Windows or Linux, and should be scheduled by a cron job (or a scheduled task for Windows).

It expects that the owner of the process that is invoking the commands has SSH access to the git repository.

# Running

Running `extrahop-backup -?` will output a list of flags:

```
-apikey string
        Your API Key for the ExtraHop host.
-gitdir string
        Optional. Directory to do a git clone into. (default "C:\\Users\\YourUser\\AppData\\Local\\Temp")
-gitrepo string
        Git repository to store backups into.
-host string
        URL to the ExtraHop host. E.g. https://extrahop01.example.com.
-v    Alias to -version.
-verbose
        Output verbose details.
-version
        Print version information.
```
An example invocation:

    extrahop-backup -host "http://extrahop01.example.com" -apikey "2378ab76677100f78362acc7387382ab" -gitrepo "git@gitlab.example.com:configbackups/extrahop-config.git"

Binary downloads can be found [on the release page](https://github.com/mhenderson-so/extrahop-backup/releases).