const tag = (strings, ...values) => strings[0] + values.length;
console.log(tag`a${1}b`);
