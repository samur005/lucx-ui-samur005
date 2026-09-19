# Updates + vk-hash

Panel Update / `x-ui update` downloads AlexeyLCP release binary and wipes a custom-built x-ui.
GitHub Sync fork with Discard wipes vkhash.go.
`git merge upstream/main` keeps the patch if you keep vkhash.go and EnsureVkHashes.

Fork has no Releases; install.sh still pulls AlexeyLCP tarball.
Need Go 1.27+ to build this tree. VPS currently has Go 1.22.

Re-apply:
```
curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/main/scripts/apply-vkhash.sh | bash
```
