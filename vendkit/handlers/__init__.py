"""Reference handlers for the handler protocol (handler-protocol spec).

Each module is an executable (`python3 -m vendkit.handlers.<name>`) that
reads one intent document (JSON) on stdin, delivers it to its vendor's API,
and prints `key=value` facts on stdout. Exit 0 = delivered; nonzero =
infrastructure failure. These ship with the framework as the first-class
GitHub / Azure DevOps support — but any executable honouring the protocol
can replace them without an engine change (DR-0014).
"""
