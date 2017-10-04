
/***************************** Helpers *****************************/

function getAPIKey() {
  cookies = document.cookie.split(';');
  for (var i = 0; i < cookies.length; i++) {
      c = cookies[i];
      while (c.charAt(0) === ' ') c = c.substring(1, c.length);
      if (c.indexOf("token=") === 0) {
        return c.substring(6, c.length);
      }
  }
  return null;
}

function sendMessage(data, callback, callbackErr){
  $.ajax({
    url: 'http://localhost:8080/req',
    type: 'post',
    data: data,
    headers: {
        Authorization: 'Bearer ' + getAPIKey()
    },
    success: callback,
    error: callbackErr
  })
}

/***************************** Setup *****************************/

$(document).ready(function(){
  // Add highlighting current item functionality to the nav bar
  $(".nav_item").click(function(){
    if ($(this).hasClass("multi")) return;
    $(".active").removeClass("active");
    $(this).addClass("active");
  });
  $(".side_menu_item").click(function(){
    $(".active").removeClass("active");
    $(this).closest(".nav_item").addClass("active");
  })

  $("#settings_button").click(loadSettings);
  $("#flare_button").click(loadFlare);
  $("#filter_button").change(filterCheckList);
  $("#submit_flare").click(submitFlare);

  setupHomePage()
});

function setupHomePage() {
  // By default, display the general status page
  loadGeneralStatus();

  // Load the version and hostname data into the top bar
  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "version"
  }), function(data, status, xhr) {
    $("#version").append(data.Major + "." + data.Minor + "." + data.Patch);
  });

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "hostname"
  }), function(data, status, xhr) {
    $("#hostname").append(JSON.stringify(data))
  });

  // Regularily check if agent is running
  setInterval(checkStatus, 2000);
}

function checkStatus() {
  $.ajax({
    url: 'http://localhost:8080/req',
    type: 'post',
    data: JSON.stringify({
      req_type: "ping",
      data: "none"
    }),
    headers: {
        Authorization: 'Bearer ' + getAPIKey()
    },
    success: function(data, status, xhr) {
      $("#agent_status").html("Connected <br>to Agent");
      $("#agent_status").css({
        "background": 'linear-gradient(to bottom, #89c403 5%, #77a809 100%)',
        "background-color": '#89c403',
        "border": '1px solid #74b807',
        "text-shadow": '0px 1px 0px #528009',
        'left': '-150px'
      })
    },
    error: function() {
      $("#agent_status").html("Not connected<br> to Agent");
      $("#agent_status").css({
        "background": 'linear-gradient(to bottom, #c62d1f 5%, #f24437 100%)',
        "background-color": '#c62d1f',
        "border": '1px solid #d02718',
        "text-shadow": '0px 1px 0px #810e05',
        'left': '-180px'
      })
    }
  });
}

/***************************** Status *****************************/

function loadGeneralStatus() {
  $(".page").css("display", "none");
  $("#general_status").css("display", "block");

  // Clear the page and add the loading sign (this request can take a few seconds)
  $("#general_status").html('<i class="fa fa-spinner fa-pulse fa-3x fa-fw center"></i>');

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "status",
    payload: "general"
  }), function(data, status, xhr){
    $('#general_status').html(data);
  }, function(){
    $('#general_status').html("<span class='center'>An error occurred.</span>");
  });
}

function loadCollectorStatus(){
  $(".page").css("display", "none");
  $("#collector_status").css("display", "block");
  $("#collector_status").html('<i class="fa fa-spinner fa-pulse fa-3x fa-fw center"></i>');

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "status",
    payload: "collector"
  }), function(data, status, xhr){
    $('#collector_status').html(data);
  }, function(){
    $('#collector_status').html("<span class='center'>An error occurred.</span>");
  });
}

/***************************** Logs *****************************/
function getLog(name){
  $(".page").css("display", "none");
  $("#logs").css("display", "block");

  $("#logs").html('<i class="fa fa-spinner fa-pulse fa-3x fa-fw center"></i>');


  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "log",
    payload: name
  }), function(data, status, xhr){
    // Remove newline at the start
    if (data.substring(0, 4) == "<br>") data = data.substring(4, data.length);

    // Initially load a maximum number of lines (but allow for loading more)
    data = trimData(data);

    $("#logs").html('<div class="log_title">' + name +  '.log' +
                    '<span class="custom-dropdown"> <select id="log_view_type">' +
                    '<option value="recent_first" selected>Most recent first</option>' +
                    '<option value="old_first">Oldest first</option>' +
                    '</select></span></div>' +
                    '<div class="log_data">' + data + ' </div>');
    $("#log_view_type").change(function(){
      changeLogView(name);
    });
  }, function(){
    $('#logs').html("<span class='center'>An error occurred.</span>");
  });
}

function changeLogView(name) {
  if ($("#log_view_type").val() == "old_first") type = "log-no-flip"
  else type = "log";

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: type,
    payload: name
  }), function(data, status, xhr){
    if (data.substring(0, 4) == "<br>") data = data.substring(4, data.length);
    data = trimData(data);

    $(".log_data").html(data);
  });
}

var extraData;
function trimData(data) {
  linesToLoad = 200;
  i = -1;

  // Find the index of the 150th occurrence of <br>
  while (linesToLoad > 0 && i < data.length) {
    i = data.indexOf("<br>", i);
    if (i < 0) break;
    linesToLoad--; i++;
  }

  if (i > 0) {
    extraData = data.substring(i+3, data.length);
    data = data.substring(0, i-1);

    // Add a way to load more
    data += "<br><a href='javascript:void(0)' onclick='loadMore()' class='load_more'> Load more </a>";
  }
  return data;
}

function loadMore() {
  data = $(".log_data").html();

  // Remove the load more button
  i = data.lastIndexOf("<a href=");
  data = data.substring(0, i);

  // Add the next 150 lines
  $(".log_data").html(data + trimData(extraData));
}

/***************************** Settings *****************************/

function loadSettings() {
  $(".page").css("display", "none");
  $("#settings").css("display", "block");

  $('#settings').html('<textarea id="settings_input"></textarea>' +
                      '<div id="submit_settings">Save</div>');
  $("#submit_settings").click(submitSettings);

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "config_file"
  }), function(data, status, xhr){
    $('#settings_input').val(data);
  }, function(){
    $('#settings').html("<span class='center'>An error occurred.</span>");
  });
}

function submitSettings() {
  settings = $("#settings_input").val();

  sendMessage(JSON.stringify({
    req_type: "set",
    data: "config_file",
    payload: settings
  }), function(data, status, xhr){
    resClass = "success"; symbol = "fa-check";
    resMsg = "Restart agent <br> to see changes";

    if (data != "Success") {
      console.log(data);
      resClass = "unsuccessful"; symbol = "fa-times";

      if (data.includes("permission denied")) resMsg = "Permission <br> denied";
      else resMsg = "An error <br> occurred";
    }

    $("#submit_settings").append('<i class="fa ' + symbol + ' fa-lg ' + resClass + '"></i><div class="msg">' + resMsg + '</div>');
    $("." + resClass + ", .msg").delay(3000).fadeOut("slow");
  });
}

/***************************** Edit Check Configs *****************************/

// only load the config files once
var loaded = false;
function loadChecks() {
  $(".page").css("display", "none");
  $("#checks_settings").css("display", "block");

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "conf_list"
  }), function(data, status, xhr){
    if (typeof(data) == "string") {
      if (data.includes("Empty directory:")) {
        // Handle the directory being empty
        path = data.substr(data.indexOf(":") + 1);
        $("#checks_interface").html("<span class='center'>" + path + " is empty.</span>");
      } else {
        // Handle some other error
        $("#checks_interface").html("<span class='center'>A problem occurred. " + data + "</span>");
      }
      return
    }

    $("#checks_interface").html("<span class='center'> Select a check configuration file. </span>");

    if (loaded) return;
    data.sort();
    data.forEach(function(item){
      if (item.substr(item.length - 4) == "yaml"){
        $("#checks_list").append('<a href="javascript:void(0)" onclick="showCheck(\'' + item + '\')" class="check enabled">' +  item + '</a>');
      } else if (item.substr(item.length - 7) == "default") {
        $("#checks_list").append('<a href="javascript:void(0)" onclick="showCheck(\'' + item + '\')" class="check default">' +  item + '</a>');
      }
    });
    $(".default").css("display", "none");
    loaded = true;
  }, function(){
    $('#checks_interface').html("<span class='center'>An error occurred.</span>");
    $("#checks_list").html("");
    loaded = false;
  });
}

function showCheck(name) {
    sendMessage(JSON.stringify({
      req_type: "fetch",
      data: "check_config",
      payload: name
    }), function(data, status, xhr){
      $("#checks_interface").html('<div id="check_title"> Editing: ' + name + '</div>' +
                                  '<div id="submit_check">Save</div>' +
                                  '<div id="reload_check">Reload</div>' +
                                  '<textarea id="check_input">' + data + '</textarea>');
      $("#submit_check").click(submitCheckSettings);
      $("#reload_check").click(reloadCheck);
    });
}

function submitCheckSettings() {
  settings = $("#check_input").val();
  name = $("#check_title").html().slice(10);

  sendMessage(JSON.stringify({
    req_type: "set",
    data: "check_config",
    payload: name + " " + settings
  }), function(data, status, xhr){
    resClass = "success"; symbol = "fa-check";
    resMsg = "Reload check <br> to see changes";
    if (data != "Success") {
      console.log(data);
      resClass = "unsuccessful"; symbol = "fa-times";
      if (data.includes("permission denied")) resMsg = "Permission <br> denied";
      else resMsg = "An error <br> occurred";
    }
    $("." + resClass + ", .msg").remove();
    $("#submit_check").append('<i class="fa ' + symbol + ' fa-lg ' + resClass + '"></i><div class="msg">' + resMsg + '</div>');
    $("." + resClass + ", .msg").delay(3000).fadeOut("slow");
  });
}

function reloadCheck() {
  name = $("#check_title").html().slice(10);

  sendMessage(JSON.stringify({
    req_type: "set",
    data: "reload_check",
    payload: name.substr(0, name.length - 5) // remove the .yaml ending
  }), function(data, status, xhr){
    $("." + resClass + ", .msg").remove();
    $("#reload_check").append('<div class="msg">' + data + '</div>');
    $(".msg").delay(4000).fadeOut("slow");
  });
}

function filterCheckList() {
  val = $("#filter_button").val();
  switch (val) {
    case "enabled":
      $(".default").css("display", "none");
      $(".enabled").css("display", "inline-block");
      break;
    case "default":
      $(".enabled").css("display", "none");
      $(".default").css("display", "inline-block");
      break;
  }
}

/***************************** See Running Checks *****************************/

function seeRunningChecks() {
  $(".page").css("display", "none");
  $("#running_checks").css("display", "block");

  runningChecks = {}
  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "running_checks"
  }), function(data, status, xhr){
    if (data == null) {
      $("#running_checks").html("No checks ran yet.");
      return
    }

    data.sort();
    data.forEach(function(name){
      if (runningChecks.hasOwnProperty(name)) runningChecks[name] += 1;
      else runningChecks[name] = 1;
    });

    $("#running_checks").html('<table id="running_checks_table">' +
                              '<tr> <th>Check Name</th>' +
                              '<th class="l_space">Number of Instances</th></tr> ' +
                              '</table>' +
                              '<div id="running_checks_info"> See Collector Status for more information.</div>');

    for (check in runningChecks) {
      $("#running_checks_table").append('<tr> <td>' + check + '</td>' +
                                        '<td class="l_space">' + runningChecks[check] + '</td></tr> ');
    }
  }, function() {
      $("#running_checks").html("An error occurred.");
  });
}

/***************************** Run a Check *****************************/

function loadAddCheck() {
  $(".page").css("display", "none");
  $("#add_check").css("display", "block");

  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "checks_list"
  }), function(data, status, xhr){
    if (typeof(data) == "string") {
      if (data.includes("Empty directory:")) {
        path = data.substr(data.indexOf(":") + 1);
        $("#_description").html("No checks to run - " + path + " is empty.");
      } else {
        $("#add_check_description").html("A problem occurred. " + data );
      }
      return
    }

    $("#add_check_description").html("To begin, please select a check to add.");
    $("#add_check_interface").html("<div class='check_files_list'></div>");

    data.sort();
    data.forEach(function(item){
      if (item.substr(item.length - 2) != "py") return;
      $(".check_files_list").append('<a href="javascript:void(0)" onclick="showCheckFile(\'' +
                                    item.substr(0, item.length - 3) + '\')" class="check_file">' +  item + '</a>');
    });

  }, function() {
    $("#add_check_description").html("An error occurred.");
    $("#add_check_interface").html("");
  });
}

function showCheckFile(fileName) {
  // See if theres any config files associated with this check
  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "conf_list"
  }), function(data, status, xhr){

    if (typeof(data) == "string") {
      if (data.includes("Empty directory:")) {
        createNewConfigFile(fileName, );
      } else {
        $("#add_check_description").html("A problem occurred. " + data);
      }
      return
    }

    enabled = false;
    associatedConfigs = [];
    data.forEach(function(item){
      if (item.substr(0, fileName.length + 1) == fileName + ".") {
        // Check if it's already an enabled check
        if (item.substr(item.length - 4) == "yaml") {
           enabled = true;
           $("#add_check_description").html("This check is already enabled; you can change it's configuration in Edit Check Configs.");
        } else {
          associatedConfigs.push(item);
        }
      }
    });

    if (!enabled) createNewConfigFile(fileName, associatedConfigs);
  }, function(){
    $("#add_check_description").html("An error occurred.");
    $("#add_check_interface").html("");
  });
}

function createNewConfigFile(fileName, associatedConfigs) {
  $("#add_check_description").html("Please create a new configuration file for this check below.");
  $(".check_files_list").html("<div class='check_file selected'>" + fileName + ".yaml </span>");
  $("#add_check_interface").append('<textarea id="edit_check_file">Add your configuration here.</textarea>' +
                                   '<div id="save_run_check">Save and<br>Test</div>');
  $("#save_run_check").click(function(){
    $("#save_run_check").css("pointer-events", "none");
    runNewCheck(fileName + ".yaml");
  });

  if (!associatedConfigs || associatedConfigs.length == 0) return;

  $("#add_check_description").append(" Associated configuration files are available to help you.");
  for (i = 0; i < associatedConfigs.length; i++) {
    $(".check_files_list").append('<a href="javascript:void(0)" onclick="showAssociated(\'' + associatedConfigs[i] +
                                  '\')" class="check_file example"> See ' +  associatedConfigs[i] + '</a>');
  }
}

function runNewCheck(name) {
  // Save the new configuration file
  settings = $("#edit_check_file").val();
  sendMessage(JSON.stringify({
    req_type: "set",
    data: "check_config",
    payload: name + " " + settings
  }), function(data, status, xhr){

    if (data != "Success") {
      // TODO
      // Error writing new file
      // if (data.includes("permission denied")) ....
    }

    // TODO:  Something saying "new check added!"

    runCheck(name, "#add_check");
  });
}

function runCheck(name, page) {
  // Run the check
  sendMessage(JSON.stringify({
    req_type: "set",
    data: "run_check_once",
    payload: name.substr(0, name.length-5) // remove the .yaml
  }), function(data, status, xhr){

    $(page).append("<div id='popup'>" + data + "<div class='exit'>x</div></div>");
    $(".exit").click(function() {
      $("#popup").remove();
      $(".exit").remove();
      $("#save_run_check").css("pointer-events", "auto");
    })

  });
}

function showAssociated(name) {
  $(".example").css("pointer-events", "none");
  $("#add_check").append("<div id='popup'>" +
                            "<div id='associated_title'>" + name + ": </div>" +
                            "<textarea readonly='readonly' id='associated'></textarea>" +
                            "<div class='exit'>x</div>" +
                          "</div>");

  $(".exit").click(function() {
    $("#popup").remove();
    $(".exit").remove();
    $(".example").css("pointer-events", "auto");
  })

  // Fetch the configuration file
  sendMessage(JSON.stringify({
    req_type: "fetch",
    data: "check_config",
    payload: name
  }), function(data, status, xhr){
    $("#associated").append(data);
  });
}


/***************************** Flare *****************************/

function loadFlare() {
  $(".page").css("display", "none");
  $("#flare").css("display", "block");
}

function submitFlare() {
  ticket = $("#ticket_num").val();
  if (ticket == "") ticket = "0";

  email = $("#email").val();
  regex = /\S+@\S+\.\S+/;   // string - @ - string - . - string
  if ( !regex.test(email) ) {
      $("#flare_response").html("Please enter a valid email address.");
      return;
  }

  sendMessage(JSON.stringify({
    req_type: "set",
    data: "flare",
    payload: email + " " + ticket
  }), function(data, status, xhr){

    // TODO (?) ask for confirmation

    $("#flare_response").html(data);
    $("#ticket_num").val("");
    $("#email").val("");
  }, function(){
    $('#flare_response').html("<span class='center'>An error occurred.</span>");
  });
}