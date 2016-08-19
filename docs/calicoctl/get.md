get.md> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../calicoctl.md) main page.

# User reference for 'calicoctl get' commands

This sections describes the `calicoctl get` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl get' command

Run `calicoctl get --help` to display the following help menu for the 
calicoctl get command.

```
Set the ETCD server access information in the environment variables
or supply details in a config file.

Display one or many resources identified by file, stdin or resource type and name.

Possible resource types include: policy

By specifying the output as 'template' and providing a Go template as the value
of the --template flag, you can filter the attributes of the fetched resource(s).

Usage:
  calicoctl get ([--tier=<TIER>] [--hostname=<HOSTNAME>] [--scope=<SCOPE>] (<KIND> [<NAME>]) |
                 --filename=<FILENAME>)
                [--output=<OUTPUT>] [--config=<CONFIG>]


Examples:
  # List all policy in default output format.
  calicoctl get policy

  # List a specific policy in YAML format
  calicoctl get -o yaml policy my-policy-1

Options:
  -f --filename=<FILENAME>     Filename to use to get the resource.  If set to "-" loads from stdin.
  -o --output=<OUTPUT FORMAT>  Output format.  One of: yaml, json.  [Default: yaml]
  -t --tier=<TIER>             The policy tier.
  -n --hostname=<HOSTNAME>     The hostname.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.  This is only valid
                               for BGP peers and is used to indicate whether the peer is a global peer
                               or node-specific.
```

Examples:

```

```
[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calicoctl/docs/calicoctl/get.md?pixel)](https://github.com/igrigorik/ga-beacon)
