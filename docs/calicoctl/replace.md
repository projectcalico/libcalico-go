replace.md> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../calicoctl.md) main page.

# User reference for 'calicoctl replace' commands

This sections describes the `calicoctl replace` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl replace' command

Run `calicoctl replace --help` to display the following help menu for the 
calicoctl replace command.

```
Set the ETCD server access information in the environment variables
or supply details in a config file.

Replace a resource by filename or stdin.

If replacing an existing resource, the complete resource spec must be provided. This can be obtained by
$ calicoctl get -o yaml <TYPE> <NAME>

Usage:
  calicoctl replace --filename=<FILENAME> [--config=<CONFIG>]

Examples:
  # Replace a policy using the data in policy.yaml.
  calicoctl replace -f ./policy.yaml

  # Replace a pod based on the YAML passed into stdin.
  cat policy.yaml | calicoctl replace -f -

Options:
  -f --filename=<FILENAME>     Filename to use to replace the resource.  If set to "-" loads from stdin.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
```

Examples:

```

```
[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calicoctl/docs/calicoctl/replace.md?pixel)](https://github.com/igrigorik/ga-beacon)
