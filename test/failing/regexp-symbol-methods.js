const r = /a/g;
console.log(typeof r[Symbol.match], typeof r[Symbol.replace], typeof r[Symbol.search], typeof r[Symbol.split]);
try { console.log(r[Symbol.match]("aba").join(",")); } catch (e) { console.log(e.name); }
try { console.log(r[Symbol.replace]("aba", "X")); } catch (e) { console.log(e.name); }
try { console.log(r[Symbol.search]("aba")); } catch (e) { console.log(e.name); }
try { console.log(r[Symbol.split]("aba").join(",")); } catch (e) { console.log(e.name); }
