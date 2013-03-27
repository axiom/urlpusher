var conn, sendMessage, osdText;

$(function() {
	var frames = $('iframe');
	var currentFrame = 0;
	var imgHolders = $('.imgholder > img');
	var currentHolder = 0;
	var osd = $('#osd > div');
	var osdTimer;

	osd.parent().hide();
	frames.hide();
	imgHolders.hide();

	frames.on('load', function() {
		if (this.src != "") {
			swapTo(this);
		}
	});

	imgHolders.on('load', function() {
		if (this.src != "") {
			swapTo(this);
		}
	});

	osdText = function(text, delay) {
		if (osdTimer) {
			clearTimeout(osdTimer);
		}

		osd.html(text).parent().show();

		if (osd.fitText) {
			osd.fitText(0.8);
		}

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

			if (message.Type == "url") {
				loadURL(message.Payload);
			} else if (message.Type == "img") {
				loadIMG(message.Payload);
			} else if (message.Type == "text") {
				osdText(message.Payload);
			} else if (message.Type == "reload") {
				if (conn) {
					conn.close();
				}
				var currentLocation = window.location;
				window.location = currentLocation;
			}
		};
	}

	sendMessage = function(type, payload) {
		conn.send(JSON.stringify({"Type": type, "Payload": payload }));
	};

	function nextFrame() {
		if (currentFrame) {
			return frames.first();
		} else {
			return frames.last();
		}
	}

	function nextIMGHolder() {
		if (currentHolder) {
			return imgHolders.first();
		} else {
			return imgHolders.last();
		}
	}

	function swapTo(element) {
		frames.fadeOut();
		imgHolders.fadeOut();
		$(element).fadeIn();

		frames.each(function() {
			if (this != element) {
				$(this).removeAttr('src');
			} else {
				currentFrame = 1 - currentFrame;
			}
		});

		imgHolders.each(function() {
			if (this != element) {
				$(this).removeAttr('src');
			} else {
				currentHolder = 1 - currentHolder;
			}
		});
	}

	function loadURL(url) {
		var next = nextFrame();
		next.attr('src', url);
	}

	function loadIMG(url) {
		var next = nextIMGHolder();
		next.attr('src', url);
	}

	connect();
});
