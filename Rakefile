require 'rbconfig'

EXE_NAME="gomeboycolor"

task :default => :build

def detect_platform 
	if RbConfig::CONFIG['host_os'] == "mswin32"
		return "windows_#{RbConfig::CONFIG['host_cpu']}".downcase
	else
		require 'sys/uname'
		os=Sys::Uname.sysname
		machine=Sys::Uname.machine
		return "#{os}_#{machine}".downcase
	end
end


@build_platform=detect_platform
@version="localbuild"


task :run, [:ars] do |t, args|
	puts "Running gomeboycolor for #{@build_platform}..."

	case @build_platform.intern
	when :darwin_x86_64
		Rake::Task["run_darwin"].invoke(args[:ars])
	else
		abort("Unsupported platform #{@build_platform}")
	end
end

#main build task, pass a platform and version string
task :build, [:version] => [:clean] do |t, args|
	if args[:version].nil? then
		@version="localbuild_" + @build_platform
	else
		@version = args[:version] + "_" + @build_platform
	end

	puts "Building gomeboycolor (version = #{@version}) for #{@build_platform}..."

	case @build_platform.intern
	when :windows_x86_64
		Rake::Task["build_windows"].invoke
	when :linux_x86_64
		Rake::Task["build_linux"].invoke
	when :linux_i386
		Rake::Task["build_linux"].invoke
	when :linux_i686
		Rake::Task["build_linux"].invoke
	when :darwin_x86_64
		Rake::Task["build_darwin"].invoke
	else
		abort("Unsupported platform #{@build_platform}")
	end
end

task :build_linux => [:setgopath, :set_cgo_flags, :get_go_deps] do
	puts "Packaging for #{@build_platform} (static linked binary)"
	ENV["CGO_LDFLAGS"] += " -Wl,-Bstatic -lGLEW -lglfw -Wl,-Bdynamic"
	puts "Reset CGO_LDFLAGS to #{ENV["CGO_LDFLAGS"]}"
	sh "#{construct_build_command(@build_platform, @version, EXE_NAME)}"
end

task :build_darwin => [:setgopath, :set_cgo_flags, :get_go_deps] do
	puts "Packaging for #{@build_platform} (dymanic linked binary)"
	sh construct_build_command(@build_platform, @version, EXE_NAME)
	sh "mkdir target/#{@build_platform}/bin && mv target/#{@build_platform}/#{EXE_NAME} target/#{@build_platform}/bin/"
	sh "cp -a #{Dir.pwd}/dist/#{@build_platform}/pkg/* target/#{@build_platform}/"
end

task :build_windows => [:setgopath, :set_cgo_flags, :get_go_deps] do |t, args|
	puts "Packaging for #{@build_platform} (dymanic linked binary)"
	sh construct_build_command(@build_platform, @version, EXE_NAME+".exe")
	sh "cp -a #{Dir.pwd}/dist/#{@build_platform}/pkg/* target/#{@build_platform}/"
end

task :run_darwin, [:prog_args] => [:setgopath, :set_cgo_flags, :get_go_deps_no_download] do |t, args|
	ENV["DYLD_LIBRARY_PATH"] = "#{Dir.pwd}/dist/#{@build_platform}/pkg/lib"
	sh "#{construct_run_command} #{args[:prog_args]}"
end

task :clean do
	puts "Cleaning target dir"
	sh "rm -rf ./target/*"
end

task :setgopath do
	puts "Setting GOPATH to #{Dir.pwd}"
	ENV["GOPATH"] = Dir.pwd
end

task :set_cgo_flags do
	ENV["CGO_CFLAGS"] = "-I#{Dir.pwd}/dist/#{@build_platform}/include"
	ENV["CGO_LDFLAGS"] = "-L#{Dir.pwd}/dist/#{@build_platform}/lib"

	puts "Set CGO_CFLAGS to #{ENV["CGO_CFLAGS"]}"
	puts "Set CGO_LDFLAGS to #{ENV["CGO_LDFLAGS"]}"
end

task :get_go_deps do
	get_deps(true)
end

task :get_go_deps_no_download do
	get_deps(false)
end


def get_deps(download)
	flag = " "
	if download then
		flag += "-d"
	end

	puts "Getting GO dependencies"
	sh "go get#{flag} code.google.com/p/freetype-go/freetype/truetype"
	sh "go get#{flag} github.com/go-gl/gl"
	sh "go get#{flag} github.com/go-gl/glfw"
end

def construct_build_command(platform, version, exename) 
	return "go build -a -o target/#{platform}/#{exename} -ldflags=\"-X main.VERSION #{version}\" src/gbc.go src/debugger.go src/config.go"  
end

def construct_run_command
	return "go run -ldflags=\"-X main.VERSION Run.local\" src/gbc.go src/debugger.go src/config.go"  
end
