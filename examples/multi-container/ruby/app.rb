# Copyright 2023 Google, LLC.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'sinatra'
require 'sequel'

set :bind, '0.0.0.0'
set :port, 8080

# Configure a connection pool that connects to the proxy via TCP
def connect_tcp
    Sequel.connect(
        adapter: 'postgres',
        host: ENV["INSTANCE_HOST"],
        database: ENV["DB_NAME"],
        user: ENV["DB_USER"],
        password: ENV["DB_PASS"],
        pool_timeout: 5,
        max_connections: 5,
    )
end

DB = connect_tcp()


get '/' do
    # Connect to the database and get the current time
    DB["SELECT NOW()"].all
end
