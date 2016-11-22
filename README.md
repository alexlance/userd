# userd

A daemon to administrate user accounts. Using a git repository of JSON user definitions

# Usage

    userd --realm production --repo https://github.com/someone/users/
    
# Users repo should have one json file per person, eg jsmith.json:

    {
      "username": "jsmith",
      "comment": "Jane Smith",
      "groups": ["admin", "sudo"],
      "realms": ["production","development"],
      "ssh_keys": [
          "ssh-ed25519 AAAAC3NzaKYCoqgI7JQGXzMQ jsmith@home"
      ]
    }
