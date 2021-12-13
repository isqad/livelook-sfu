const path = require("path");

module.exports = {
  entry: "./src/home.js",
  output: {
    path: path.join(__dirname, "/dist"),
    filename: "home.js"
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

