### Ansible Role

The purpose of this role is to import the variables from a JinjaFx Vault namespace into the `jinjafx_vault` dict to make them accessible within your template.

#### playbook.yml

```yaml
- hosts: all
  gather_facts: no
  connection: local

  vars_prompt:
    - name: jinjafx_vault_user
      prompt: "Vault User"
      private: "no"
    - name: jinjafx_vault_password
      prompt: "Password"
      private: "yes"

  tasks:
    - ansible.builtin.import_role:
        name: jinjafx_vault
      vars:
        vault_url: "https://jinjafx.vault.url:8443"
        vault_timeout: 5 # default
        vault_verify: true # default        
        vault_user: "{{ jinjafx_vault_user }}"
        vault_password: "{{ jinjafx_vault_password }}"
        vault_namespace: "<namespace>"

    - ansible.builtin.template:
        src: "template.j2"
        dest: "{{ inventory_hostname }}.txt"
```

#### template.j2

```jinja2
{{ jinjafx_vault.<variable> }}
```
