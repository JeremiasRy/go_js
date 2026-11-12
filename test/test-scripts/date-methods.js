const d = new Date(0);
console.log(typeof d.toUTCString, typeof d.toDateString, typeof d.toTimeString, typeof d.getTimezoneOffset);
try { console.log(d.toUTCString()); } catch (e) { console.log(e.name); }
console.log(typeof Date, Date.length, Date.UTC.length);
