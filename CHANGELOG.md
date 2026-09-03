## CHANGELOG

### 0.2.0 - September 3, 2026
- Session timeout has been changed to an idle timeout (default is 15mn)
- Added command line option `-idle` to change idle timeout
- Added command line option `-rlimit` to change login rate limit (default is 3/15mn)
- Changed `/v1/chage` duration values to `hr`, `dy`, `wk`, `mh`, `yr`
- Added API method `/v1/<user>/expire` to force user password change

### 0.1.0 - September 2, 2026
- Initial release


[0.1.0]: https://github.com/cmason3/jinjafx_vault/compare/v0.1.0...v0.2.0
