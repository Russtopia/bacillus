#!/bin/bash

inFile="${1/.go/}"
visFile="${inFile}-vis.gv"

#grep -o "\.[a-zA-Z_]*\$[0-9]*" "$inFile"-vis.gv | sort | uniq
grep -o "#gv:.*" "$inFile.go" | cut -f2 -d: | \
while read -r expr; do sed -i ${expr} "${visFile}"; done

