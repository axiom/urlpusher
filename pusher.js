var conn, sendMessage, osdText;

$(function() {
	var frames = $('iframe');
	var osd = $('#osd > div');
	var osdTimer;
	osd.parent().hide();

	frames.on('load', function() {
		if (this.src) {
			swap();
		}
	});

	osdText = function(text, delay) {
		if (osdTimer) {
			clearTimeout(osdTimer);
		}

		osd.html(text).parent().show();
		osd.fitText(0.8);

		osdTimer = setTimeout(function() {
			osd.parent().fadeOut();
		}, delay || 5000);
	}

	function connect() {
		var reconnectTimer;
		var host = 'ws://' + window.location.host + window.location.pathname.replace(/\/[^\/]+$/, '/') + 'pusher';

		conn = new WebSocket(host);

		conn.onclose = function(evt) {
			console.log("onclose", evt);
			osdText('disconnected');

			// Set a reconnect timer...
			if (reconnectTimer) {
				clearTimeout(reconnectTimer);
			}

			reconnectTimer = setTimeout(connect, 3000);
		};

		conn.onopen = function(evt) {
			console.log("onopen", evt);
			osdText('connected');
		};

		conn.onerror = function(evt) {
			console.log("onerror", evt);
			osdText('error');
		};

		conn.onmessage = function(evt) {
			console.log("onmessage", evt.data);

			var message = JSON.parse(evt.data);

			if (message.type == "url") {
				loadURL(message.payload);
			} else if (message.type == "text") {
				osdText(message.payload);
			} else if (message.type == "reload") {
				if (conn) {
					conn.close();
				}
				var currentLocation = window.location;
				window.location = currentLocation;
			}
		};
	}

	sendMessage = function(type, payload) {
		conn.send(JSON.stringify({ "type": type, "payload": payload }));
	};

	function currentFrame() {
		return $('iframe.current');
	}

	function nextFrame() {
		return $('iframe.next');
	}

	function swap() {
		var current = currentFrame();
		var next = nextFrame();

		next.removeClass('loading');
		current.removeClass('current'); current.addClass('next');
		next.removeClass('next'); next.addClass('current');
	}

	function loadURL(url) {
		var next = nextFrame();
		next.addClass('loading');
		next.attr('src', url);
	}

	connect();
});
