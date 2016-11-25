const request = superagent

fetch = (fetchMetrics) => () =>
  fetchMetrics((res, err) => {
  if (err != null) {
    console.log(err)
    return
  }
  if (res.body == null) {
    return
  }
  const s = calculateMetrics(state, res.body)
  window.requestAnimationFrame(() => {
    document.getElementById("95th").innerText=s["95th"]
    document.getElementById("requests").innerText=s["requests"]
    document.getElementById("success").innerText=s["success"]
    document.getElementById("errors").innerText=s["errors"]
  })
  console.log(res.body)
})

var aggregatorInterval;
document.getElementById("set-aggregator").onclick = () => {
  if (aggregatorInterval != null) {
    clearInterval(aggregatorInterval)
  }
  var aggregatorUrl = document.getElementById("aggregator-url").value
  if (aggregatorInterval == "") {
    return
  }
  const fetchMetrics = (results) => {
    request.
      get(aggregatorUrl).
      end((err, res) => {
        results(res, err)
      })
  }
  aggregatorInterval = setInterval(fetch(fetchMetrics), 1000)
}

var coordinatorUrl = ""
document.getElementById("set-coordinator").onclick = () => {
  coordinatorUrl = document.getElementById("coordinator-url").value
}

const renderAttackerList = (attackers) => {
    const tmpl = (url) => `<li>${url}</li>`
    const innerHTML = attackers.map(tmpl).join("")
    requestAnimationFrame(() => {
      document.getElementById("list-attackers").innerHTML = innerHTML
    })
}

var attackers = []
document.getElementById("add-attacker").onclick = () => {
  const attackerUrl = document.getElementById("add-attacker-url").value
  if (attackers.indexOf(attackerUrl) != -1) {
    console.log("Attacker is already added")
    return
  }

  attackers.push(attackerUrl)

  renderAttackerList(attackers)
}

document.getElementById("start").onclick = () => {
  console.log("start")
}

document.getElementById("stop").onclick = () => {
  console.log("stop")
}

const state = {
  requests: 0,
  success: 0,
  status_codes: {},
  errors: 0,
  "95th": 0,
  lastUpdated: {},
}

const calculateMetrics = (state, metrics) => {
  const aggr = metrics.map((m,i) => {
    if (state.lastUpdated[i] == null || state.lastUpdated[i] < metrics.latest) {
      return {
        requests: m.requests,
        status_codes: m.status_codes,
      }
    }
  })

  if (aggr.length == 0) {
    return state;
  }

  state.requests = state.requests + aggr.map((m) => m.requests).reduce((a,b) => a + b, 0)

  const statusCodes = aggr.map((m) => m.status_codes)
  .reduce((mem, curr) => {
    Object.keys(curr).forEach((key) => {
        if (mem[key] == null) {
          mem[key] = curr[key]
        } else {
          mem[key] = mem[key] + curr[key]
        }
    })
    return mem
  }, {})

  state.success = state.success + statusCodes["200"]

  const collect95th = metrics.map((m) => m.latencies["95th"]).reduce((a,b) => (a + b)/2, 0)
  state["95th"] = (state["95th"] + collect95th) / 2
  state.errors = state.requests - state.success

  return state
}
