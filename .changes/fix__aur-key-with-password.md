# Fix AUR packaging

Previous attempt to release the package to AUR failed, because the signing key was password protected,
and the release process did not support password input. 
This change removes the password from the signing key, allowing the release process to complete successfully.
