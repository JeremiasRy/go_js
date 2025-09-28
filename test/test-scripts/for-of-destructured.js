const users = [
    { id: 1, name: "Alice", food: "spaghetti" },
    { id: 2, name: "Bob" }
];

for (const { name, food } of users) {
    console.log(`${name} favorite food ${food}`);
}