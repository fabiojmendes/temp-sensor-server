/* Requires the Docker Pipeline plugin */
pipeline {
  agent { docker { image 'golang:1.19.1-alpine' } }
  stages {
    stage('build producer') {
      steps {
        environment {
          GOOS = 'linux'
          GOARCH = 'arm'
          GOARM = 6
        }
        sh 'go build -o build/ ./src/tsproducer'
      }
    }
    stage('build consumer') {
      steps {
        environment {
          GOOS = 'linux'
          GOARCH = 'amd64'
        }
        sh 'go build -o build/ ./src/consumer'
      }
    }
  }
}
