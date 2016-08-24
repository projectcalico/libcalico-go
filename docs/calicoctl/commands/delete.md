> ![warning](../images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../README.md) main page.

# User reference for 'calicoctl delete' commands

This sections describes the `calicoctl delete` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl delete' command

Run `calicoctl delete --help` to display the following help menu for the 
calicoctl delete command.

```
Set the ETCD server access information in the environment variables
or supply details in a config file.

Delete a resource identified by file, stdin or resource type and name.

Usage:
  calicoctl delete (([--tier=<TIER>] [--hostname=<HOSTNAME>] [--scope=<SCOPE>] <KIND> <NAME>) |
                    --filename=<FILE>)
                   [--skip-not-exists] [--config=<CONFIG>]

Examples:
  # Delete a policy using the type and name specified in policy.yaml.
  calicoctl delete -f ./policy.yaml

  # Delete a policy based on the type and name in the YAML passed into stdin.
  cat policy.yaml | calicoctl delete -f -

  # Delete policy in the default tier with name "foo"
  calicoctl delete policy foo

  # Delete policy in the tier "bar" with name "foo"
  calicoctl delete policy --tier=bar foo

Options:
  -s --skip-not-exists         Skip over and treat as successful, resources that don't exist.
  -f --filename=<FILENAME>     Filename to use to delete the resource.  If set to "-" loads from stdin.
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
[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/libcalico-go/docs/calicoctl/commands/delete.md?pixel)](https://github.com/igrigorik/ga-beacon)
