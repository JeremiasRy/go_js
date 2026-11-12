function tag(strings, ...values) { console.log(strings[0], strings.raw[0], values.join(",")); }
tag`a\n${1}b${2}`;
