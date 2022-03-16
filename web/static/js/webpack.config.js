const path = require("path");

module.exports = {
  entry: {
    home: "./src/home.js",
    admin: "./src/admin.js",
    broadcastShow: "./src/broadcast_show.js",
  },
  output: {
    path: path.join(__dirname, "/dist"),
    filename: '[name].bundle.js'
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: {
          loader: "babel-loader"
        },
      },
      {
        test: /\.css$/,
        use: ["style-loader", "css-loader"]
      }
    ]
  }
};

