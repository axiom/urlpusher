$(function() {
	var conn, sendMessage;
	var container = $('#directory');

	function connect() {
		var reconnectTimer;
		var host = 'ws://' + window.location.host + window.location.pathname.replace(/\/[^\/]+$/, '/') + 'pusher';

		conn = new WebSocket(host);

		conn.onclose = function(evt) {
			console.log("onclose", evt);

			// Set a reconnect timer...
			if (reconnectTimer) {
				clearTimeout(reconnectTimer);
			}

			reconnectTimer = setTimeout(connect, 3000);
		};

		conn.onopen = function(evt) {
			console.log("onopen", evt);
			sendMessage("list");
		};

		conn.onerror = function(evt) {
			console.log("onerror", evt);
		};

		conn.onmessage = function(evt) {
			var message = JSON.parse(evt.data);

			if (message.Type == "list") {
				listEntries(message.Payload);
			}
		};
	}

	function listEntries(entries) {
		// Clear any previous listing before loading up the new one.
		container.children().remove();

		for (var i = 0; i < entries.length; i++) {
			// Populate the entry template
			entries[i].isURLType = "url" == entries[i].Type;
			entries[i].isImgType = "img" == entries[i].Type;
			var entry = ich.entry(entries[i]);

			container.append(entry);
		}
	}

	function updateEntry(entry) {
		sendMessage("set", {
			ID: entry.prop('id'),
			Type: entry.find('[name=type]').val(),
			URL: entry.find('[name=url]').val(),
			DumbDuration: entry.find('[name=duration]').val(),
			Name: entry.find('[name=name]').val()
		});
	}

	sendMessage = function(type, payload) {
		conn.send(JSON.stringify({ "type": type, "payload": payload }));
		console.log(JSON.stringify({ "type": type, "payload": payload }));
	};

	connect();

	// Setup click and key handlers
	$('input.create').on('click', function() {
		sendMessage("set", {});
	});

	$(document).on('click', 'input.destroy', function(e) {
		sendMessage('delete', $(this).parent().prop('id'));
	});

	$(document).on('click', 'input[type=submit]', function(e) {
		updateEntry($(this).parent());
	});

	container.on('keypress', 'input', function(e) {
		if (e.which == 13) {
			updateEntry($(this).parent());
		}
	});

});
