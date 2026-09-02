## CHANGELOG

### 0.2.0 - In Development
- Vault reset now re-enables root acount if disabled
- Session timeout has been changed to an idle timeout (default is 15mn)
- Added command line option `-idle` to change idle timeout
- Added command line option `-rl` to change login rate limit (default is 3/15mn)
- Changed `chage` duration values to `hr`, `dy`, `wk`, `mh`, `yr`
- Need to add something when we add accounts so they expire immediately for userpass - maybe /expired?

### 0.1.0 - September 2, 2026
- Initial release


[0.1.0]: https://github.com/cmason3/jinjafx_vault/compare/v0.1.0...v0.2.0
