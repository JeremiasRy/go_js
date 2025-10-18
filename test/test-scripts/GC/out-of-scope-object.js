function createAndLeaveObject() {
    const localObject = {
        name: "Temporary",
        value: "value"
    };
    console.log("Object created inside function.");
    console.log(localObject)
}

for (let i = 0; i < 10000; i++) {
    createAndLeaveObject()
}