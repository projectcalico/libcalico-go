apply.md> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../calicoctl.md) main page.

# User reference for 'calicoctl apply' commands

This sections describes the `calicoctl apply` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl apply' command

Run `calicoctl apply --help` to display the following help menu for the 
calicoctl apply command.

```
Set the ETCD server access information in the environment variables
or supply details in a config file.

Apply a resource by filename or stdin.  This creates a resource
if it does not exist, and replaces a resource if it does exist.

Usage:
  calicoctl apply --filename=<FILENAME> [--config=<CONFIG>]

Examples:
  # Apply a policy using the data in policy.yaml.
  calicoctl apply -f ./policy.yaml

  # Apply a policy based on the JSON passed into stdin.
  cat policy.json | calicoctl apply -f -

Options:
  -f --filename=<FILENAME>     Filename to use to apply the resource.  If set to "-" loads from stdin.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
```

Examples:

```

```
[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calicoctl/docs/calicoctl/apply.md?pixel)](https://github.com/igrigorik/ga-beacon)
