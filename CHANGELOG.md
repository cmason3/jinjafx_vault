## CHANGELOG

### [v0.3.2] - September 18, 2026
- Various cosmetic enhancements to the web interface
- Updated the Ansible Role with some improvements

### [v0.3.1] - September 17, 2026
- Fixed an issue where error messages were blank in the web interface
- Differentiated between Authentication Failed and Verification Failed
- Improvements to the web interface to make it usable.

### [v0.3.0] - September 16, 2026
- Removed `expires` on login response as it was pointless being an idle timeout
- Added initial support (read-only) for a web interface to manage JinjaFx Vault
- Login rate limit now applies to remote host regardless of user
- Added `debug.go` to enable debug builds to assist in development
- You can no longer remove all roles from a user
- Added support for `/v1/whoami` to API

### [v0.2.3] - September 10, 2026
- You can no longer delete a namespace unless it is empty

### [v0.2.2] - September 10, 2026
- Removed historic rate limits after successfully logging in
- Upgraded golang.org/x/crypto v0.56.0 => v0.57.0
- Upgraded golang.org/x/sys v0.47.0 => v0.48.0
- Upgraded golang.org/x/term v0.45.0 => v0.46.0

### [v0.2.1] - September 3, 2026
- Fixed an issue if `<jinjafx.vault>` doesn't exist
- Fixed a segmentation violation when calling `/v1/logout` due to a race condition

### [v0.2.0] - September 3, 2026
- Session timeout has been changed to an idle timeout (default is 15mn)
- Added command line option `-idle` to change idle timeout
- Added command line option `-rlimit` to change login rate limit (default is 3/15mn)
- Changed `/v1/chage` duration values to `hr`, `dy`, `wk`, `mh`, `yr`
- Added API method `/v1/<user>/expire` to force user password change

### v0.1.0 - September 2, 2026
- Initial release


[v0.3.2]: https://github.com/cmason3/jinjafx_vault/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/cmason3/jinjafx_vault/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.3...v0.3.0
[v0.2.3]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/cmason3/jinjafx_vault/compare/v0.1.0...v0.2.0
