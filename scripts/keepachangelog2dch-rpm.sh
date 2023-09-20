#!/bin/bash

for varname in PACKAGE_AUTHOR PACKAGE_EMAIL PACKAGE_NAME PACKAGE_FORMAT; do
  if [ -z "${!varname}" ]; then
    echo "$varname must be set" > 2
    exit 1
  fi
done

# from https://stackoverflow.com/a/246128
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# process the keepachangelog markdown file
gawk \
  -v author="$PACKAGE_AUTHOR" \
  -v email="$PACKAGE_EMAIL" \
  -v package="$PACKAGE_NAME" \
  -v format="$PACKAGE_FORMAT" \
  -f $SCRIPT_DIR/dch_rpm.awk -e \
  'BEGIN {
    if (format == "dch") { dch = 1; }
    else if (format == "rpm") { rpm = 1; }
    else { print "unknown format " format; exit 1 }
   }
   /^\[/ { if (dch) { print_footer_dch(author, email, date) }; exit }
   /^## \[/ {
   if (p && dch) { print_footer_dch(author, email, date) }
   version = keepachangelog_version($2)
   date = keepachangelog_date($4)
   if (dch) { print_header_dch(package, version, "UNRELEASED", "low") }
   if (rpm) { print_header_rpm(author, email, date) }
   p = 1
   next
   }
   p && NF {
    if (dch) { print_line_dch($0) }
    if (rpm) { print_line_rpm($0) }
   }
  ' "$1"
