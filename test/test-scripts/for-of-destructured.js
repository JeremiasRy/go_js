const users = [
    { id: 1, name: "Alice" },
    { id: 2, name: "Bob" }
];

for (const { name } of users) {
    console.log(name);
}