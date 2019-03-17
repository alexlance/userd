userd v1.13
===========


Debian user management from a git repository
--------------------------------------------

    Gather user account information from a specified git repository, then
    administrate the user accounts and ssh access across multiple servers.


### Installation

Download the latest `userd` binary from the github releases tab and put the
file into your PATH. Ensure the file is executable. Then add it as a cron
jon or systemd timer on every server that you want administrated.

    # /etc/crontab
    */15 * * * * root userd --realm development --repo https://github.com/someone/users


### Git repository

When the application is run, `userd` clones your user account git repository
into memory.

The git repository contains your centralized list of users. It then checks that
list and adds or removes user accounts from the server as required.

    The git repository should be locked down to prevent unauthorized write access.

If the git repo contains ssh public keys `userd` will keep each user's
`~/.ssh/authorized_keys` up to date with those keys. The user's groups and
other account details may be specified in the git repo as well.

Since all user administration is performed by changing a git repo, there is a
solid audit trail behind every server access that is granted for every user.
This seems to keep auditors happy. We make use of Pull Requests so that a user
may kick-start their own request for access.


### Realms

Each server belongs to a realm. The realm name is arbitrary, and it is up to
you to decide what sort of name to use. The realm is used by `userd` to decide
whether a user should or shouldn't exist on that server (ie in that realm).

You might decide to define your realms quite broadly like: green, orange, red.
Or take a fine-grained approach by using hostnames or IP addresses.

The level of granularity is up to you. We use AWS Instance Profile names across
hundreds of servers for medium-grained access. This works well when a
particular application is spread across multiple servers that all have the
same Instance Profile name.


### User definition format

The git repository that contains all the user accounts should contain multiple
JSON files, one JSON file per user. Each JSON file should have the file suffix
`.json`.

The contents of one file should represent one user, and define all the
servers and groups that the user belongs to, eg here is `jane.smith.json`:

    {
      "username": "jsmith",
      "comment": "Jane Smith",
      "realms": [
        "production",
        "development",
        "test-*"
      ],
      "groups": [
        "admin",
        "sudo:development"
      ],
      "shell": "/bin/bash",
      "password": "[encrypted-password-hash]",
      "ssh_keys": [
          "ssh-ed25519 AAAAC3NzaKYCoqgI7JQGXzMQ jsmith@home"
      ]
    }

In this example Jane will be added to all servers that are part of the
"production" or "development" realms, *she will also be added to every
single realm whose name begins with "test-"*.

Jane will be in the admin group across all the realms, but will only be in
the sudo group for the development realm.

The encrypted password hash can be generated using the `openssl` tool, eg:

    openssl passwd -1
    Password: [enter a new password]
    Verifying - Password: [enter it again]
    $1$uxa.NCuA$Y6FQJaSRaRtfK1OUcOD5P1

Most fields in the JSON file can be omitted if they are not desired. If the
realms are set to an empty array `[]` then that user account will be removed
from every server that `userd` is administrating.
