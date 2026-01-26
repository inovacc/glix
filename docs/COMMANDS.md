# Command Tree

```
glix [module]
+-- auto-update                              # Manage automatic update settings
|   +-- config                               # Configure auto-update settings
|   +-- disable                              # Disable automatic updates
|   +-- enable                               # Enable automatic updates
|   +-- now                                  # Run update check immediately
|   \-- status                               # Show auto-update status
+-- install                                  # Install a Go module
+-- list                                     # List all installed modules
+-- monitor                                  # Check all installed modules for avail...
+-- remove                                   # Remove an installed Go module
+-- report                                   # Show details about an installed module
+-- service                                  # Manage the glix background service
|   +-- install                              # Install the glix service on the system
|   +-- start                                # Start the glix service
|   +-- status                               # Show the glix service status
|   +-- stop                                 # Stop the glix service
|   \-- uninstall                            # Remove the glix service from the system
+-- update                                   # Update an installed Go module to the ...
\-- version                                  # Print version information
```
