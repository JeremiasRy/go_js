async function fetchUser() {
    return new Promise(resolve => {
        setTimeout(() => {
            resolve({ id: 1, name: "Grace" });
        }, 1000);
    });
}

async function main() {
    const user = await fetchUser();
    console.log(user.name);
}

main();