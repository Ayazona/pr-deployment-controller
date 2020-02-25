const path = require("path");
const glob = require("glob");

const outputPath = path.resolve(__dirname, "public");
const publicPath = "/term/__static/";

module.exports = (env, { mode }) => {
  const config = {
    context: path.resolve(__dirname),

    entry: {
      main: "./src/index.js"
    },
    output: {
      filename: `[name].js`,
      path: outputPath,
      publicPath
    },
    resolve: {
      // Allow us to import from e.g. 'core/js/something', as well as from node_modules.
      modules: glob.sync("**/static/").concat(["node_modules"]),
      extensions: [".js"]
    },
    module: {
      rules: [
        {
          test: /\.css$/,
          use: ["style-loader", "css-loader"]
        }
      ]
    },
    plugins: []
  };

  return config;
};
