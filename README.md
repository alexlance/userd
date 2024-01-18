userd v1.21
===========


Debian/Ubuntu/CentOS user management
------------------------------------

User account and ssh key management on Linux.

Userd gathers user account definitions from a git repository, then
administrates the user accounts and ssh access for Linux servers.


### Installation

Download the latest `userd` binary from the github releases tab and put the
file into a $PATH. Ensure the file is executable. Then add it as a cron
jon or systemd timer on every server that you want to administrate.

    # /etc/crontab
    */15 * * * * root userd --realm development --repo https://github.com/somewhere/ourusers


### Git repository

When the application is run, `userd` clones a git repository into memory.

The repository should contain a list of users. `Userd` checks that list and
adds or removes user accounts from a server as required.

    The user account git repository should be locked down to prevent unauthorized write access.

If the git repo contains ssh public keys, `userd` will keep each user's
`~/.ssh/authorized_keys` up to date with those keys. Each user's group membership
(and other account details) will be updated as well.

Since all user administration is performed by git repository commits, there is
a solid audit trail behind every access that is granted to every user. A Pull
Request may be created by unauthorized users to kick-start a grant for access.


### Realms

Realms are used to define access perimeters. The actual realm names are
arbitrary and defined by you.

Each server belongs to a realm. The realm name is used by `userd` to decide
whether a user account should or shouldn't exist on a server (ie is _this_ user
in _that_ realm?).

You might decide to define your realms quite broadly eg: green, orange, red.
Or take a fine-grained approach eg: using each server's hostname or IP address
as a separate realm.

For example, we use *AWS Instance Profile names* as our realms. This works
because our particular applications are spread across multiple servers that may
all have the same Instance Profile name.


### User definition format

The git repository that contains all the user accounts should contain multiple
JSON files, **one JSON file per user**. Each JSON file should have the file suffix
`.json`.

The contents of one file should define all the servers and groups that one user
belongs to, eg here is `jane.smith.json`:

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
*production* or *development* realms, she will also be granted access to
**every realm** whose name begins with "test-".

Jane will be in the *admin* group for every realm, but will only be in
the *sudo* group for the *development* realm.

The encrypted password hash can be generated using the `openssl` tool, eg:

    openssl passwd -1
    Password: [enter a new password]
    Verifying - Password: [enter it again]
    $1$uxa.NCuA$Y6FQJaSRaRtfK1OUcOD5P1

Most fields in the JSON file can be omitted if they are not desired. If the
*realms* are set to an empty array `[]` then that user account will be removed
from every server that `userd` is administrating.
