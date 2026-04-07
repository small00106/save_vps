# Release 目录

这个目录只保存发布脚本和说明，不保存正式二进制。

- `build-release.sh`：在 Linux 构建机上生成 GitHub Release 资产
- `dist/`：构建输出目录，默认被 `.gitignore` 忽略

正式发布产物统一上传到 GitHub Releases，不再提交进仓库。
