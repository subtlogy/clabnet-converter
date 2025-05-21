# clabnet-converter

clabnet-converter is a tool for converting real physical network device configurations into [containerlab](https://containerlab.dev/) topologies and node configs.  
It **automatically detects device connections and peer links** from raw configs, making it easy to recreate physical networks in a virtual environment.

Inspired by: [JANOG51 Meeting](https://www.janog.gr.jp/meeting/janog51/lab/)

## Features

- Converts physical configs (Cisco IOS/NX-OS; other network OS support planned) into containerlab topology and configs
- Extracts hostnames, interfaces, and link information from raw configs using interface descriptions
- Cleans and normalizes configs for virtual deployment
- Generates topology YAML and per-node config files
- Includes Ansible playbook for config push

## How connection information is extracted

**Important**  
To automatically determine how devices are connected, each physical interface in your config **must include a description in the following format**:  
 
```
description <peer-hostname>:<peer-interface>
```

For example:

```
description test02:Ethernet1/1
```

This tells the converter that this interface connects to interface `Ethernet1/1` on the device whose hostname is `test02`.  
This convention must be used for all relevant interfaces you want to be automatically linked in the generated topology.

## Sample

A ready-to-use sample is provided in the repository root and `conf_samples/` directory.  
The sample demonstrates a simple two-node topology.

```
clabnet-converter/
├── sample.yml
└── conf_samples/
    ├── n1.cfg
    └── n2.cfg
```

To try the tool with these files, simply run:

```
go run ./cmd/labbuild sample.yml
```

## Usage

### 1. Prepare a lab definition YAML (example: `sample.yml`)

**Be sure to include:**
- The lab name (`name:`)
- Each device's node name, `kind`, `image`, and `raw_config` path  
  (These fields are **required** for all devices.)

```yaml
name: my-lab
topology:
  nodes:
    node1:
      kind: cisco_cat9kv
      image: vrnetlab/cisco_cat9kv:17.12.01p
      raw_config: conf_samples/n1.cfg
    node2:
      kind: cisco_n9kv
      image: vrnetlab/cisco_n9kv:9300-10.5.3
      raw_config: conf_samples/n2.cfg
```

### 2. Run the converter

```shell
go run ./cmd/labbuild sample.yml
# → creates ./topology.yml and ./new_configs/node1.cfg etc.

# (optional) Specify output file
go run ./cmd/labbuild -t my-topo.yml sample.yml
# → creates ./my-topo.yml
```

If you need to add or modify topology options (such as `env`, `binds`, etc.) in the generated topology.yml, edit the file manually before deploying with containerlab.

### 3. Deploy with containerlab

```shell
containerlab deploy -t topology.yml
```

### 4. Set LAB_NAME environment variable

```shell
export LAB_NAME=$(awk -F: '/^name:/ {gsub(/^[ \t]+|["'\'']/,"",$2); print $2}' sample.yml)
# → LAB_NAME=my-lab
```

### 5. Patch the inventory for Ansible

```shell
sed -i '/cisco_cat9kv:/,/hosts:/s/^\(\s*vars:\)$/\1\n        ansible_network_os: ios/' ./clab-${LAB_NAME}/ansible-inventory.yml
sed -i '/cisco_n9kv:/,/hosts:/s/^\(\s*vars:\)$/\1\n        ansible_network_os: nxos/' ./clab-${LAB_NAME}/ansible-inventory.yml
```

### 6. Push configs with Ansible

```shell
ansible-playbook play.yml
```

## Roadmap
- Support for other network OS (Arista, Juniper, etc.)

## References

This project was heavily inspired by the ideas and approach described in the following presentation:
- JANOG51 Meeting: [Containerlabを使用した商用環境と同等な検証環境の作成とユースケースについて (PDF)](https://www.janog.gr.jp/meeting/janog51/wp-content/uploads/2022/12/janog51-lab-nakagawa-shima.pdf)
