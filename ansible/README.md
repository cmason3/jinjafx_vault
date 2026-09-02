### template.j2
```jinja2
{{ lookup("jinjafx_vault", "<namespace>", "<variable>") }}
```

### playbook.yml
```yaml
---
- hosts: all
  gather_facts: no
  force_handlers: true # to Force JinjaFx Vault Logout
  connection: local

  vars_prompt:
    - name: jinjafx_vault_user
      prompt: "Vault User"
      private: "no"
    - name: jinjafx_vault_password
      prompt: "Password"
      private: "yes"

  vars:
    jinjafx_vault_url: "https://jinjafx.vault.url:8443"
    jinjafx_vault_timeout: 5 # default
    jinjafx_vault_verify: true # default

  tasks:
    - ansible.builtin.import_role:
        name: jinjafx_vault

    - ansible.builtin.template:
        src: "template.j2"
        dest: "{{ inventory_hostname }}.txt"
