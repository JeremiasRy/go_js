try { new Set({}); console.log("set-ok"); } catch (e) { console.log("set", e.name); }
try { new Map([1]); console.log("map-ok"); } catch (e) { console.log("map", e.name); }
