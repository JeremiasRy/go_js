const user = {
    id: 101,
    firstName: "Alice",
    lastName: "Johnson",
    age: 30,
    city: "Wonderland"
};

const { id, firstName, ...otherInfo } = user;


const result = {
    ...otherInfo
};

console.log(`${result.id}${result.firstName}${result.age}${result.city}${result.lastName}`);

const newObj = {
    ...user,
    id: 102
}

console.log(`${newObj.id}${newObj.firstName}${newObj.lastName}${newObj.age}${newObj.city}`)

