(() => {
  const timer = setInterval(() => {
    const target = document.body?.innerText || ''
    if (target.includes('签到')) {
      clearInterval(timer)
    }
  }, 1000)
})()
