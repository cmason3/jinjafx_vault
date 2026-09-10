## CHANGELOG

### 0.2.2 - September 10, 2026
- Removed historic rate limits after successfully logging in
- Upgraded golang.org/x/crypto v0.56.0 => v0.57.0
- Upgraded golang.org/x/sys v0.47.0 => v0.48.0
- Upgraded golang.org/x/term v0.45.0 => v0.46.0

### 0.2.1 - September 3, 2026
- Fixed an issue if `<jinjafx.vault>` doesn't exist
- Fixed a segmentation violation when calling `/v1/logout` due to a race condition

### 0.2.0 - September 3, 2026
- Session timeout has been changed to an idle timeout (default is 15mn)
- Added command line option `-idle` to change idle timeout
- Added command line option `-rlimit` to change login rate limit (default is 3/15mn)
- Changed `/v1/chage` duration values to `hr`, `dy`, `wk`, `mh`, `yr`
- Added API method `/v1/<user>/expire` to force user password change

### 0.1.0 - September 2, 2026
- Initial release


[0.2.2]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/cmason3/jinjafx_vault/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cmason3/jinjafx_vault/compare/v0.1.0...v0.2.0
