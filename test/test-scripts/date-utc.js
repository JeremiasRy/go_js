// M3: Fixed UTC/offset inputs avoid wall-clock and local-time dependencies.
const epoch = new Date(0);
console.log("epoch", epoch.getTime(), epoch.toISOString(), epoch.getUTCFullYear(), epoch.getUTCMonth(), epoch.getUTCDate(), epoch.getUTCDay());
console.log("offset", new Date("2020-01-02T03:04:05.006+02:00").toISOString());
console.log("utc", new Date(Date.UTC(2000, 1, 29, 12, 34, 56, 789)).toISOString());
console.log("rollover", new Date(Date.UTC(2020, 12, 1)).toISOString(), new Date(Date.UTC(99, 0, 1)).getUTCFullYear());
const changed = new Date("2020-01-01T00:00:00.000Z");
changed.setUTCFullYear(2024);
changed.setUTCMonth(1);
changed.setUTCDate(29);
const timestamp = changed.setUTCHours(23, 59, 58, 7);
console.log("setters", timestamp === changed.getTime(), changed.toISOString(), changed.getUTCHours(), changed.getUTCMinutes(), changed.getUTCSeconds(), changed.getUTCMilliseconds());
changed.setUTCSeconds(60);
console.log("second-rollover", changed.toISOString());
changed.setUTCMinutes(1);
changed.setUTCMilliseconds(999);
console.log("remaining-setters", changed.getUTCMinutes(), changed.getUTCMilliseconds());
console.log("parse", Date.parse("1970-01-01T00:00:00.000Z"), new Date(-1).toISOString());
