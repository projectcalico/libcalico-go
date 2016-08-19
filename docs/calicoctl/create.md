> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../calicoctl.md) main page.

# User reference for 'calicoctl create' commands

This sections describes the `calicoctl create` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl create' command

Run `calicoctl create --help` to display the following help menu for the 
calicoctl create command.

```
Set the ETCD server access information in the environment variables
or supply details in a config file.

Create a resource by filename or stdin.

Usage:
  calicoctl create --filename=<FILENAME> [--skip-exists] [--config=<CONFIG>]

Examples:
  # Create a policy using the data in policy.yaml.
  calicoctl create -f ./policy.yaml

  # Create a policy based on the JSON passed into stdin.
  cat policy.json | calicoctl create -f -

Options:
  -f --filename=<FILENAME>     Filename to use to create the resource.  If set to "-" loads from stdin.
  -s --skip-exists             Skip over and treat as successful any attempts to create an entry that
                               already exists.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
```

Examples:

```

```
[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calicoctl/docs/calicoctl/create.md?pixel)](https://github.com/igrigorik/ga-beacon)
