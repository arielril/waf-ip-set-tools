# waf-ip-set-tools

Wrapper to use the functionality from AWS WAFv2 via cli

## Installation

```sh
go install -v github.com/arielril/waf-ip-set-tools@latest
```

## Usage

```sh
$ waf-ip-set-tools -h
Usage of waf-ip-set-tools:
  -action string
    	Action to execute in the IP Set. Values: [add-ip remove-ip clear]
  -data string
    	IP CIDR list, separated with commas (','). Ex: "cidr_1,cidr_2,cidr_3"
  -file string
    	File with a list of IP CIDR
  -id string
    	IP Set ID
  -name string
    	IP Set Name
  -profile string
    	AWS Profile
  -region string
    	AWS Region
  -scope string
    	IP Set Scope (default "CLOUDFRONT")
```

### Add a list of IP CIDR to an IP Set

```sh
$ waf-ip-set-tools -profile 'aws-profile' -region 'us-east-1' -id 'ipset-id' \
  -name 'ipset-name' -action 'add-ip' -data '187.114.175.159/32,191.5.67.33/32'
```

### Add a list of IP CIDR to an IP Set from a file

```sh
$ waf-ip-set-tools -profile 'aws-profile' -region 'us-east-1' -id 'ipset-id' \
  -name 'ipset-name' -action 'add-ip' -file '/path/to/file'
```

### Remove a list of IP CIDR from an IP Set

```sh
$ waf-ip-set-tools -profile 'aws-profile' -region 'us-east-1' -id 'ipset-id' \
  -name 'ipset-name' -action 'remove-ip' -data '187.114.175.159/32,191.5.67.33/32'
```

### Remove a list of IP CIDR from an IP Set from a file

```sh
$ waf-ip-set-tools -profile 'aws-profile' -region 'us-east-1' -id 'ipset-id' \
  -name 'ipset-name' -action 'remove-ip' -file '/path/to/file'
```

### Clear an IP Set

```sh
$ waf-ip-set-tools -profile 'aws-profile' -region 'us-east-1' -id 'ipset-id' \
  -name 'ipset-name' -action 'clear'
```
