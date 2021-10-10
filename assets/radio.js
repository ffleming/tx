console.log("hey radio")

function togglePower() {
  state = getState()
  if($("#switch").text() === "On") {
    state.on = false
  } else {
    state.on = true
  }
  writeState(state)
}

function getState() {
  state = {
    on: $("#switch").text() === "On",
    frequency: $("#tx-frequency").text(),
    dial: {
      selected: $("#dial-selected").text(),
      stations: [],
    },
    directory: {
      stations: [],
    },
  }

  $("#dial-body .callsign").each((_, el) => {
    state.dial.stations = state.dial.stations.concat($(el).text())
  })

  all_stations = []
  $("#directory-body .list-item:not(#directory-add-container)").each((_, el) => {
    station = {
      callsign: $(el).find(".callsign").text(),
      frequency:  $(el).find(".frequency").text(),
      url:  $(el).find(".url").text(),
      info:  $(el).find(".info").text(),
    }
    all_stations = all_stations.concat(station)
  })

  state.directory.stations = all_stations
  return state
}

function writeState(state) {
  $.post("/radio", JSON.stringify(state), function(resp){
    draw(resp)
  }, "json")
}

function draw(state) {
  $("#directory-add-frequency").val("")
  $("#directory-add-info").val("")
  $("#directory-add-url").val("")
  $("#directory-add-callsign").val("")
  $("#tx-frequency").val("")

  $("#switch").text(state.on ? "On" : "Off")
  $("#dial-selected").text(state.dial.selected)
  $("#toggle-power").removeClass("fa-toggle-on fa-toggle-off").addClass("fas fa-toggle-" + (state.on ? "on" : "off"))
  $(".list-item:not(#directory-add-container)").each((_, el) => {
    el.remove()
  })
  $("#tx-frequency").val(state.frequency)

  state.dial.stations.forEach(callsign => {
    station = state.directory.stations.find(s => s.callsign == callsign)
    if(!!station) {
      addDialRow(station)
    } else {
      console.log("Error: station " + callsign + " not in directory")
    }
  })
  state.directory.stations.forEach(station => {
    isCallInDial = !!(state.dial.stations.find(cs => cs == station.callsign))
    addDirectoryRow(station, isCallInDial)
  })
  bind()
}

function addDirectoryRow(obj, hidden) {
  html = `<div class="list-item ` + (hidden ? "hidden" : "") + `">
            <div class ="col-sm-1"><span class="callsign">` + obj["callsign"] + `</span></div>
            <div class ="col-sm-1"><span class="frequency">` + obj["frequency"] + `</span></div>
            <div class ="col-sm-4"><span class="url">` + obj["url"] + `</span></div>
            <div class ="col-sm-4"><span class="info">` + obj["info"] + `</span></div>
            <div class ="col-sm-1">
              <i class="fas fa-plus-circle directory-send-to-dial"></i>
            </div>
            <div class ="col-sm-1">
              <i class="fas fa-trash directory-remove"></i>
            </div>
          </div>`

  $("#directory-add-container").before(html)
}

function addDialRow(obj) {
  html = `<div class="list-item">
              <div class="col-sm-3"><span class="callsign">` + obj.callsign + `</span></div>
              <div class="col-sm-2">` + obj.frequency + `</div>
              <div class="col-sm-5">` + obj.info + `</div>
              <div class="col-sm-1">
                <i class="fas fa-play dial-tune"></i>
              </div>
              <div class="col-sm-1">
                <i class="fas fa-minus-circle dial-remove"></i>
              </div>
            </div>`
  $("#dial-body").append(html)
}

function createStation(e) {
  state = getState()
  station = {
    frequency: $("#directory-add-frequency").val(),
    info: $("#directory-add-info").val(),
    url: $("#directory-add-url").val(),
    callsign: $("#directory-add-callsign").val(),
  }
  state.directory.stations.push(station)

  writeState(state)
}

function dialRemove(e) {
  e.preventDefault()
  callsign = $(e.target).closest(".list-item").find(".callsign").text()

  state = getState()
  if(state.dial.stations.length <= 1) {
    console.log("Can't remove last station")
    return
  }

  stations = state.dial.stations
  idx = stations.indexOf(callsign)
  if (idx > -1) {
    stations.splice(idx, 1);
  }
  state.dial.stations = stations
  if(state.dial.selected == callsign) {
    state.dial.selected == state.dial.stations[0]
  }
  writeState(state)
}

function directorySendToDial(e) {
  e.preventDefault()
  callsign = $(e.target).closest(".list-item").find(".callsign").text()

  state = getState()
  state.dial.stations.push(callsign)
  writeState(state)
}

function directoryRemove(e) {
  e.preventDefault()
  callsign = $(e.target).closest(".list-item").find(".callsign").text()

  state = getState()
  stations = state.directory.stations.filter(s => s.callsign != callsign)
  state.directory.stations = stations
  writeState(state)
}

function dialTune(e) {
  e.preventDefault()
  callsign = $(e.target).closest(".list-item").find(".callsign").text()
  state = getState()
  state.dial.selected = callsign
  writeState(state)
}

function singleBind() {
  $("#toggle-power").click(togglePower)
  $("#directory-add").click(createStation)

  $("#dial-body").sortable()
  $("#directory-body").sortable({
    items: '.list-item:not(:last)'
  })
}

function bind() {
  $(".dial-remove").click(dialRemove)
  $(".dial-tune").click(dialTune)

  $(".directory-remove").click(directoryRemove)
  $(".directory-send-to-dial").click(directorySendToDial)
}

$(function() {
  singleBind()
  bind()
  $.get( "/radio", function( state ) {
    draw(state)
  })
});
