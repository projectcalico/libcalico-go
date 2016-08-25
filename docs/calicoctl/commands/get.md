> ![warning](../images/warning.png) This document describes an alpha release of calicoctl
>
> See note at top of [calicoctl guide](../README.md) main page.

# User reference for 'calicoctl get' commands

This sections describes the `calicoctl get` command.

Read the [calicoctl command line interface user reference](../calicoctl.md) 
for a full list of calicoctl commands.

## Displaying the help text for 'calicoctl get' command

Run `calicoctl get --help` to display the following help menu for the 
calicoctl get command.

```
Display one or many resources identified by file, stdin or resource type and name.

By specifying the output as 'go-template' and providing a Go template as the value
of the --go-template flag, you can filter the attributes of the fetched resource(s).

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
  -o --output=<OUTPUT FORMAT>  Output format.  One of: ps, wide, custom-columns=..., yaml, json,
                               go-template=..., go-template-file=...   [Default: ps]
  -t --tier=<TIER>             The policy tier.
  -n --hostname=<HOSTNAME>     The hostname.
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.  This is only valid
                               for BGP peers and is used to indicate whether the peer is a global peer
                               or node-specific.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
```

## calicoctl create

The create command is used to create a set of resources by filename or stdin.  JSON and
YAML formats are accepted.

Attempting to create a resource that already exists is treated as a terminating error unless the
`--skip-exists` flag is set.  If this flag is set, resources that already exist are skipped.
   
The output of the command indicates how many resources were successfully created, and the error
reason if an error occurred.  If the `--skip-exists` flag is set then skipped resources are 
included in the success count.

The resources are created in the order they are specified.  In the event of a failure
creating a specific resource it is possible to work out which resource failed based on the 
number of resources successfully created.

#### Examples
```
# Create a set of resources (of mixed type) using the data in resources.yaml.
# Results indicate that 8 resources were successfully created.
$ calicoctl create -f ./resources.yaml
Successfully created 8 resource(s)

# Create the same set of resources reading from stdin.
# Results indicate failure because the first resource (in this case a Tier) already exists.
$ cat resources.yaml | calicoctl apply -f -
Failed to create any resources: resource already exists: Tier(name=tier1)
```


#### Options
```
  -f --filename=<FILENAME>     Filename to use to get the resource.  If set to "-" loads from stdin.
  -o --output=<OUTPUT FORMAT>  Output format.  One of: ps, wide, custom-columns=..., yaml, json,
                               go-template=..., go-template-file=...   [Default: ps]
  -t --tier=<TIER>             The policy tier.
  -n --hostname=<HOSTNAME>     The hostname.
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.  This is only valid
                               for BGP peers and is used to indicate whether the peer is a global peer
                               or node-specific.
```

#### General options
```
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
```

#### See also
-  [Resources](resources/README.md) for details on all valid resources, including file format
   and schema
-  [Policy](resources/policy.md) for details on the Calico tiered policy model
-  [calicoctl configuration](general/config.md) for details on configuring `calicoctl` to access
   the Calico datastore.

[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/libcalico-go/docs/calicoctl/commands/get.md?pixel)](https://github.com/igrigorik/ga-beacon)
