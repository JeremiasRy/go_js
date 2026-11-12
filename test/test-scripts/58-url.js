const u = new URL("https://example.com/path?q=1");
console.log(u.hostname, u.searchParams.get("q"));
