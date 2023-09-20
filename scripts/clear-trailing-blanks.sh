#!/bin/bash
# from https://stackoverflow.com/a/7363393

awk '
    /[[:graph:]]/ {
        # a non-empty line
        # set the flag to begin printing lines
        p=1
        # print the accumulated "interior" empty lines
        for (i=1; i<=n; i++) print ""
        n=0
        # then print this line
        print
    }
    p && /^[[:space:]]*$/ {
        # a potentially "interior" empty line. remember it.
        n++
    }
' $1
