Release Artifacts
=================
- cloudnest-master-linux-amd64
- cloudnest-master-linux-arm64
- cloudnest-master-windows-amd64 (PE binary, no extension)
- cloudnest-master-windows-amd64.zip
- cloudnest-agent-linux-amd64
- cloudnest-agent-linux-arm64
- SHA256SUMS.txt

Notes
-----
1) Frontend is embedded from cloudnest/public/dist (already rebuilt).
2) Master download API expects agent binaries at runtime in ./data/binaries/.
3) If you prefer Windows .exe direct upload, use git add -f for the .exe file or just use the zip artifact.
