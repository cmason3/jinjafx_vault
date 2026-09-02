# JinjaFx Vault

JinjaFx Vault is a Secrets Manager written in Go. It was written to accompany [JinjaFx](https://github.com/cmason3/jinjafx) to provide the ability to store credentials in a secure encrypted vault, with more granular access controls than Ansible Vault. It takes its inspiration from HasiCorp Vault, but with a much reduced feature set. The vault itself is encrypted using XChaCha20-Poly1305 with a cryptographically secure 32 byte random key, which provides 256-bit encryption. User credentials are stored within the vault using the Argon2id password hashing function where `t=8`, `m=32768` and `p=4`. Alongside standard user/password credentials it also supports secure ldap to authenticate users.

JinjaFx Vault uses the concept of namespaces, which contain variables that can hold arbituary data (i.e. strings, numbers, arrays, lists, etc). Users are granted read/write or read-only access to namespaces to provide granular role based access control. All access to the vault is via a HTTP REST API, with the preference being via TLS unless you are placing a TLS based proxy in front of it (e.g. haproxy or nginx).

### Usage

```
./jinjafx_vault <action> [options]

Actions:
  -init <jinjafx.vault>         Initialise New Vault
  -reset <jinjafx.vault>        Reset Vault Credentials
  -serve <jinjafx.vault>        Start Vault Server

Options:
  -l[isten] <address>           Listen Address (default is 127.0.0.1)
  -p[ort] <port>                Listen Port (default is http/8080 or https/8443)
  -tls                          Enable Transport Layer Security
   -tls.crt <vault.crt>         TLS Certificate Chain
   -tls.key <vault.crt>         TLS Private Key
  -xff                          Use X-Forwarded-For in Logs
  -k                            Allow Insecure LDAPS

Environment Variables:
  JFX_VAULT_KEY                 JinjaFx Vault Key
```

### Installation as a Service

The following commands will install JinjaFx Vault as a system service listening on `https://localhost:8443`.

```
./jinjafx_vault -init <jinjafx.vault>
```

```
sudo tee /etc/systemd/system/jinjafx_vault.service >/dev/null <<-EOF
[Unit]
Description=JinjaFx Vault

[Service]
Environment="JFX_VAULT_KEY=<KEY>"
ExecStart=/usr/local/bin/jinjafx_vault -serve <jinjafx.vault> -tls -tls.crt <vault.crt> -tls.key <vault.key>
Restart=on-success

[Install]
WantedBy=default.target
EOF
```

```
sudo systemctl daemon-reload
sudo systemctl enable --now jinjafx_vault.service
sudo systemctl status jinjafx_vault.service
```

### User Roles

JinjaFx uses the concept of roles when defining users - the "admin" role is used to add/delete users, add/delete namespaces and other admin type functions, but it isn't allowed to add, modify or delete variables from within namespaces (it isn't even allowed to view them). The "user" role provides the opposite capabilities - it can't perform any of the admin functions, but it can get, add, update or delete variables within namespaces. The "root" user is the default admin user that is created when you initialise your vault - this account is used to add users and namespaces and then assign "rw" or "ro" permissions to namespaces on a per user basis. You can't assign the "root" user the "user" role, but you can assign the "admin" role to a normal user.


### Getting Started

Once you have initialised a new vault and are serving it over HTTP or HTTPS, you can access it via the `/v1/login` method of the REST API using the "root" account.

#### Login as "root"
```
export TOKEN=$(curl -sS -X POST -d '{ "user": "root", "password": "<password>" }' https://localhost:8443/v1/login | jq -r '.token')
```

This will result in a JSON response with a token that can be used for subsequent requests using the `X-Vault-Token` header. Using the root account we can now create a regular user "user1", a namespace "default" and then give the new user read/write access to it before logging out of the root account.

#### Create UserPass User "user1"
```
curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "password": "<password>" }' https://localhost:8443/v1/user/user1/userpass
```

#### Create Namespace "default"
```
curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/namespace/default
```

#### Assign User "user1" to Namespace "default" with Read/Write Permissions
```
curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/user1/namespaces/default/rw
```

The new "user1" account can now be used to add variable "var1" to the "default" namespace and then retrieve it.

#### Login as "user1"
```
export TOKEN=$(curl -sS -X POST -d '{ "user": "user1", "password": "<password>" }' https://localhost:8443/v1/login | jq -r '.token')
```

#### Create Variable "var1" to Namespace "default"
```
curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "data": "value" }' https://localhost:8443/v1/data/default/var1
```

#### Get Variable "var1" from Namespace "default"
```
curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/data/default/var1
```

If you are accessing JinjaFx Vault from a JinjaFx template then you can use the `lookup("jinjafx_vault", ...)` lookup function to access namespace variables - details are located at https://github.com/cmason3/jinjafx/tree/main#jinjafx-vault

There is also a JinjaFx Vault Ansible role that provides a similar `jinjafx_vault` lookup to access namespace variables - details, as well as the Ansible role are located in the `ansible/` directory of this repository.

### REST API Methods

<details>
 <summary><b>Get Audit Log</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>GET</code> <code><b>/v1/audit</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 200 | OK |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/audit
 ```

 #### Example Response
 ```json
 {
   "2026-08-24T12:44:00.925439627Z": {
     "user": "system",
     "address": "local",
     "message": "Vault Initialised"
   },
   "2026-08-24T12:46:33.183923586Z": {
     "user": "root",
     "address": "127.0.0.1",
     "message": "ADD /user/<user>"
   },
   "2026-08-24T14:09:39.32707299Z": {
     "user": "root",
     "address": "127.0.0.1",
     "message": "ADD /namespace/<namespace>"
   },
   "2026-08-24T14:10:57.545217863Z": {
     "user": "root",
     "address": "127.0.0.1",
     "message": "ADD /user/<user>/namespaces/<namespace>/rw"
   }
 }
 ```
 <hr>

</details>

<details>
 <summary><b>Get Users</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>GET</code> <code><b>/v1/users</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 200 | OK |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/users
 ```

 #### Example Response
 ```json
 {
   "root": {
     "roles": [
       "admin"
     ],
     "password": "<password hash>",
     "namespaces": null
   },
   "<user>": {
     "roles": [
       "user"
     ],
     "password": "<password hash>",
     "namespaces": {
       "default": "rw"
     }
   }
 }
 ```
 <hr>

</details>

<details>
 <summary><b>Get Namespaces</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>GET</code> <code><b>/v1/namespaces</b></code></summary>

 #### Required Roles
 - admin (All Namespaces)
 - user (User Assigned Namespaces)

 #### Required Headers
 - X-Vault-Token

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 200 | OK |
 | 401 | Not Logged In |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/namespaces
 ```

 #### Example Response (Permissions: "na" = No Access, "ro" = Read-Only, "rw" = Read/Write)
 ```json
 {
   "<namespace1>": "<permission>",
   "<namespace2>": "<permission>",
   ...
 }
 ```
 <hr>

</details>

<details>
 <summary><b>Get Namespace Variable(s)</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>GET</code> <code><b>/v1/data/&lt;namespace&gt;[/&lt;variable&gt;]</b></code>   </summary>

 #### Required Roles
 - user

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`
 - variable - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 200 | OK |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 404 | Variable Not Found |
 | 418 | User Password Expired |

 #### Example Request (Single Variable)
 ```
 curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/data/<namespace>/<variable>
 ```

 #### Example Response (Single Variable)
 ```json
 {
   "data": <value>
 }
 ```

 #### Example Request (All Variables)
 ```
 curl -sS -X GET -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/data/<namespace>
 ```

 #### Example Response (All Variables)
 ```json
 {
   "<variable1>": {
     "data": <value>
   },
   "<variable2>": {
     "data": <value>
   },
   ...
 }
 ```

</details>

<hr>

<details>
 <summary><b>Login to Vault</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/login</b></code></summary>

 #### Required Roles
 - admin
 - user

 #### Required Data
 ```json
 {
   "user": "<user>",
   "password": "<password>"
 }
 ```

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 200 | Logged In |
 | 400 | Bad Request |
 | 401 | Password Verification Failed / User Disabled |
 | 429 | Too Many Failed Attempts (3 in 15m) |

 #### Example Request
 ```
 curl -sS -X POST -d '{ "user": "<user>", "password": "<password>" }' https://localhost:8443/v1/login
 ```

 #### Example Response
 ```json
 {
   "token": "6b5ee43f-8b0f-4189-9d65-d7e9bf4534da",
   "expires": "2026-08-25T15:05:37.664964191Z"
 }
 ```
 <hr>

</details>

<details>
 <summary><b>Logout from Vault</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/logout</b></code></summary>

 #### Required Roles
 - admin
 - user

 #### Required Headers
 - X-Vault-Token

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Logged Out |
 | 401 | Not Logged In |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/logout
 ```
 <hr>

</details>

<details>
 <summary><b>Change Password for UserPass User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/chpass</b></code></summary>

 #### Required Roles
 - admin
 - user

 #### Required Headers
 - X-Vault-Token

 #### Required Data
 ```json
 {
   "old_password": "<old password>"
   "password": "<new password>"
 }
 ```

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Password Updated |
 | 400 | Bad Request |
 | 401 | Not Logged In / Password Verification Failed |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "old_password": "<old password>", "password": "<password>" }' https://localhost:8443/v1/chpass
 ```
 <hr>

</details>

<details>
 <summary><b>Create or Update UserPass User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/userpass</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Required Data
 ```json
 {
   "password": "<password>"
 }
 ```

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 201 | User Created |
 | 204 | User Modified |
 | 304 | User Not Modified |
 | 400 | Bad Request |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "password": "<password>" }' https://localhost:8443/v1/user/<user>/userpass
 ```
 <hr>

</details>

<details>
 <summary><b>Create or Update LDAP User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/ldaps</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Required Data
 ```json
 {
   "ldap_server": "<ldap server>",
   "ldap_domain": "<ldap domain>"
 }
 ```

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 201 | User Created |
 | 204 | User Modified |
 | 304 | User Not Modified |
 | 400 | Bad Request |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "ldap_server": "<ldap server>", "ldap_domain": "<ldap domain>" }' https://localhost:8443/v1/user/<user>/ldaps
 ```
 <hr>

</details>

<details>
 <summary><b>Set Expiration for UserPass User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/chage/&lt;n[hdwmy]&gt;</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | User Expiry Information Updated |
 | 304 | User Not Modified |
 | 400 | Not UserPass User |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request (Password Expires in 3 Months)
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/chage/3m
 ```
 <hr>

</details>

<details>
 <summary><b>Disable User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/disabled</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | User Disabled |
 | 304 | User Not Modified |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request (Assign Admin Role)
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/disabled
 ```
 <hr>

</details>

<details>
 <summary><b>Assign Role to User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/roles/(admin|user)</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Role Assigned |
 | 304 | User Not Modified |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request (Assign Admin Role)
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/roles/admin
 ```
 <hr>

</details>

<details>
 <summary><b>Create Namespace</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/namespace/&lt;namespace&gt;</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 201 | Namespace Created |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 409 | Namespace Already Exists |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/namespaces/<namespace>
 ```
 <hr>

</details>

<details>
 <summary><b>Assign User to Namespace</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/user/&lt;user&gt;/namespaces/&lt;namespace&gt;/(ro|rw)</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Namespace Assigned |
 | 304 | User Not Modified |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found / Namespace Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/namespaces/<namespace>/rw
 ```
 <hr>

</details>

<details>
 <summary><b>Create or Update Namespace Variable(s)</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>POST</code> <code><b>/v1/data/&lt;namespace&gt;[/&lt;variable&gt;]</b></code></summary>

 #### Required Roles
 - user

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`
 - variable - `^[a-z][a-z0-9_]*$`

 #### Required Data (Single Variable)
 ```json
 {
   "data": <value>
 }
 ```

 #### Required Data (Multiple Variables)
 ```json
 {
   "<variable1>": {
     "data": <value>
   },
   "<variable2>": {
     "data": <value>
   },
   ...
 }
 ```

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Variable(s) Updated |
 | 400 | Bad Request |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 418 | User Password Expired |

 #### Example Request (Single Variable)
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "data": "<value>" }' https://localhost:8443/v1/data/<namespace>/<variable>
 ```

 #### Example Request (Multiple Variables)
 ```
 curl -sS -X POST -H "X-Vault-Token: $TOKEN" -d '{ "<variable1>": { "data": "<value>" }}' https://localhost:8443/v1/data/<namespace>
 ```

</details>

<hr>

<details>
 <summary><b>Delete User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/user/&lt;user&gt;</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | User Deleted |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>
 ```
 <hr>

</details>

<details>
 <summary><b>Clear Expiration for UserPass User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/user/&lt;user&gt;/chage</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | User Expiry Information Updated |
 | 304 | User Not Modified |
 | 400 | Not UserPass User |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/chage
 ```
 <hr>

</details>

<details>
 <summary><b>Enable User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/user/&lt;user&gt;/disabled</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | User Enanbled |
 | 304 | User Not Modified |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 404 | User Not Found |
 | 418 | User Password Expired |

 #### Example Request (Assign Admin Role)
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/disabled
 ```
 <hr>

</details>

<details>
 <summary><b>Remove Role from User</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/user/&lt;user&gt;/roles/(admin|user)</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Role Removed |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found / Role Not Found |
 | 418 | User Password Expired |

 #### Example Request (Remove Admin Role)
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/roles/admin
 ```
 <hr>

</details>

<details>
 <summary><b>Delete Namespace</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/namespace/&lt;namespace&gt;</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Namespace Deleted |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 404 | Namespace Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/namespace/<namespace>
 ```
 <hr>

</details>

<details>
 <summary><b>Remove User from Namespace</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/user/&lt;user&gt;/namespaces/&lt;namespace&gt;</b></code></summary>

 #### Required Roles
 - admin

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`
 - user - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Namespace Removed |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges / Permission Denied |
 | 404 | User Not Found / Namespace Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/user/<user>/namespaces/<namespace>
 ```
 <hr>

</details>

<details>
 <summary><b>Delete Namespace Variable</b><br />&nbsp;&nbsp;&nbsp;&nbsp;<code>DELETE</code> <code><b>/v1/data/&lt;namespace&gt;/&lt;variable&gt;</b></code></summary>

 #### Required Roles
 - user

 #### Required Headers
 - X-Vault-Token

 #### Valid Keys
 - namespace - `^[a-z][a-z0-9_:-]*$`
 - variable - `^[a-z][a-z0-9_]*$`

 #### Response Codes
 | Code | Response |
 | :-: | :-- |
 | 204 | Variable Deleted |
 | 401 | Not Logged In |
 | 403 | Insufficient Privileges |
 | 404 | Variable Not Found |
 | 418 | User Password Expired |

 #### Example Request
 ```
 curl -sS -X DELETE -H "X-Vault-Token: $TOKEN" https://localhost:8443/v1/data/<namespace>/<variable>
 ```

</details>

<hr>

> [!CAUTION]
> There are no guarantees the code in any branch will compile or work successfully at any given time - only release tags are guaranteed to successfully compile.
