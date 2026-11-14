Promise.resolve("ok").then(async value => {
    const resolved = await Promise.resolve(value)
    console.log(resolved)
})
