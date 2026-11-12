const o = { a: 1, b: 2 };
JSON.stringify(o, function (k, v) {
    if (k === "a") { console.log(this === o, Object.keys(this).join(",")); }
    return v;
});
